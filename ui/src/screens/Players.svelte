<script>
  // Players of the server: "my stats" card (C'est moi / !me), period, search (also by an old name, and by IP for the
  // staff), all / online / new / with a note; on PC a table (playtime, sessions, ping, best score, messages…).
  import Icon from '../components/Icon.svelte';
  import CodName from '../components/CodName.svelte';
  import Seg from '../components/Seg.svelte';
  import DataTable from '../components/DataTable.svelte';
  import { get } from '../lib/api.js';
  import { app, currentServer, mapName, selectServer } from '../lib/state.svelte.js';
  import { meOn, claim } from '../lib/me.svelte.js';
  import { go, href } from '../lib/router.svelte.js';
  import { clean, agoText, fmt, dateTime, dur, pingColor } from '../lib/format.js';
  import { permsOf } from '../lib/perms.js';
  import { t } from '../lib/i18n.svelte.js';
  import { say } from '../lib/toast.svelte.js';
  import { tagLabel } from '../lib/moderation.js';

  const PAGE = 60;
  const srv = $derived(currentServer());
  const id = $derived(app.serverId);
  const me = $derived(meOn(id));
  const priv = $derived(permsOf(app.me, id).has('private'));

  let list = $state(null);
  let failed = $state(false);
  let form = $state(null);
  let query = $state('');
  let filter = $state('all');
  let period = $state('90d');
  let newKeys = $state(/** @type {Set<string>} */ (new Set()));
  /** @type {Record<string, any>} */
  let notesBy = $state({});
  /** @type {Record<string, number>} */
  let pingBy = $state({});
  let shown = $state(PAGE);
  let code = $state('');
  let claiming = $state(false);

  $effect(() => {
    const sid = id, p = period;
    if (!sid) return;
    let alive = true;
    list = null;
    failed = false;
    newKeys = new Set();
    get(`/api/servers/${sid}/players?range=${p}`)
      .then(r => { if (alive) list = r.players ?? []; })
      .catch(() => { if (alive) { list = []; failed = true; } });
    get(`/api/servers/${sid}/retention?range=${p}`)
      .then(r => { if (alive) newKeys = new Set((r.new_players ?? []).map(x => x.key)); })
      .catch(() => {});
    return () => { alive = false; };
  });

  // staff: admin notes and pings
  $effect(() => {
    const sid = id, p = period;
    notesBy = {};
    pingBy = {};
    if (!sid || !priv) return;
    let alive = true;
    get(`/api/servers/${sid}/notes`).then(r => {
      if (!alive) return;
      /** @type {Record<string, any>} */
      const by = {};
      for (const n of r.notes ?? []) {
        by[n.key] = n;
        for (const nm of String(n.names ?? '').split('|')) if (nm) by['n:' + nm] ??= n;
      }
      notesBy = by;
    }).catch(() => {});
    get(`/api/servers/${sid}/pings?range=${p}`).then(r => {
      if (!alive) return;
      /** @type {Record<string, number>} */
      const by = {};
      for (const x of r.players ?? []) by[x.guid || 'n:' + x.name_clean] = x.avg;
      for (const x of r.players ?? []) by['n:' + x.name_clean] ??= x.avg;
      pingBy = by;
    }).catch(() => {});
    return () => { alive = false; };
  });

  const noteOf = p => notesBy[p.key] ?? notesBy['n:' + p.name_clean] ?? null;
  const pingOf = p => pingBy[p.key] ?? pingBy['n:' + p.name_clean] ?? null;
  const periodText = $derived({ '7d': t('vus ces 7 derniers jours'), '30d': t('vus ces 30 derniers jours'), '90d': t('vus ces 90 derniers jours') }[period]);

  // my card: the last 7 days (else everything known in the list)
  $effect(() => {
    const sid = id, key = me?.key;
    form = null;
    if (!sid || !key) return;
    let alive = true;
    get(`/api/servers/${sid}/form?key=${encodeURIComponent(key)}`).then(f => { if (alive) form = f; }).catch(() => {});
    return () => { alive = false; };
  });

  $effect(() => { query; filter; period; shown = PAGE; });
  $effect(() => { if (filter === 'note' && !priv) filter = 'all'; });

  const kdOf = p => p.kills == null && p.deaths == null ? null : (p.kills ?? 0) / Math.max(1, p.deaths ?? 0);
  const online = $derived((list ?? []).filter(p => p.online));
  const rows = $derived.by(() => {
    const q = clean(query).trim().toLowerCase();
    return (list ?? [])
      .filter(p => filter !== 'online' || p.online)
      .filter(p => filter !== 'new' || newKeys.has(p.key))
      .filter(p => filter !== 'note' || noteOf(p))
      .filter(p => !q || (p.name_clean ?? '').toLowerCase().includes(q) || (p.aliases ?? '').toLowerCase().includes(q) || (priv && (p.ip ?? '').includes(q)))
      .sort((a, b) => (b.online ?? 0) - (a.online ?? 0) || (b.last_seen ?? 0) - (a.last_seen ?? 0));
  });

  const mine = $derived((list ?? []).find(p => p.key === me?.key));
  const kpis = $derived.by(() => {
    const c = form?.current;
    if (c && c.kills + c.deaths > 0) {
      return [
        { v: c.kd.toFixed(2), label: t('K/D · 7 j') },
        { v: fmt(c.kills), label: t('kills · 7 j') },
        { v: c.kills ? `${Math.round((100 * c.headshots) / c.kills)} %` : '—', label: t('headshots') }
      ];
    }
    if (mine && kdOf(mine) != null) {
      return [{ v: kdOf(mine).toFixed(2), label: t('K/D') }, { v: fmt(mine.kills ?? 0), label: t('kills') }, { v: fmt(mine.deaths ?? 0), label: t('morts') }];
    }
    return [];
  });

  const others = p => String(p.aliases ?? '').split(', ').filter(n => n && n !== p.name_clean);
  const COLS = $derived([
    { k: 'level', label: t('Niveau'), v: p => (p.level == null ? null : (p.prestige ?? 0) * 100 + p.level), f: p => (p.level == null ? '—' : (p.prestige ? `P${p.prestige} · ` : '') + p.level) },
    { k: 'play', label: t('Temps de jeu'), v: p => p.playtime_s ?? 0, f: p => (p.playtime_s == null ? '—' : dur(p.playtime_s)), title: t('inconnu pour les joueurs vus seulement dans le log du jeu ou par B3') },
    { k: 'sessions', label: t('Sessions'), v: p => p.sessions ?? 0 },
    { k: 'kd', label: 'K/D', v: kdOf, f: p => (kdOf(p) == null ? '—' : kdOf(p).toFixed(2)), strong: true },
    ...(priv ? [{ k: 'ping', label: t('Ping'), title: t('ping moyen sur la période'), v: pingOf, f: p => (pingOf(p) == null ? '—' : `${Math.round(pingOf(p))} ms`) }] : []),
    { k: 'best', label: t('Meilleur score'), v: p => p.best_score },
    { k: 'msg', label: t('Messages'), v: p => p.messages ?? 0 },
    ...(priv ? [{ k: 'ip', label: 'IP', text: true, v: p => p.ip }] : []),
    { k: 'first', label: t('Première apparition'), v: p => p.first_seen, f: p => dateTime(p.first_seen) },
    { k: 'last', label: t('Dernière apparition'), v: p => (p.online ? Infinity : p.last_seen), f: p => (p.online ? t('en jeu') : dateTime(p.last_seen)) }
  ]);

  async function useCode(e) {
    e.preventDefault();
    if (!code.trim() || claiming) return;
    claiming = true;
    try {
      const r = await claim(code);
      code = '';
      say(t('C\'est toi : {name} · tes stats sont épinglées', { name: clean(r.name_raw) }));
      if (r.server_id !== app.serverId && app.servers.some(s => s.id === r.server_id)) selectServer(r.server_id);
      go('joueur', r.key);
    } catch (err) {
      say(err.message);
    } finally {
      claiming = false;
    }
  }
</script>

<h1 class="title">{t('Joueurs')}</h1>

{#if me}
  <a class="me card" href={href('joueur', me.key)}>
    <span class="top">
      <span class="av">{clean(me.name_raw).charAt(0).toUpperCase() || '?'}</span>
      <span class="who"><span class="label">{t('Mes stats')}</span><span class="nm"><CodName raw={mine?.name_raw || me.name_raw} /></span></span>
      <Icon name="chevronRight" />
    </span>
    {#if kpis.length}
      <span class="kpis">
        {#each kpis as k (k.label)}<span><b>{k.v}</b><small>{k.label}</small></span>{/each}
      </span>
    {/if}
  </a>
{:else}
  <div class="hint">
    <span class="ic"><Icon name="star" size={20} /></span>
    <div class="txt">
      <b>{t('Retrouve tes stats en un tap')}</b>
      <span>{t('Ouvre ta fiche et appuie sur « C\'est moi », ou tape !me en jeu pour recevoir ton lien.')}</span>
      <form class="code" onsubmit={useCode}>
        <label class="sr-only" for="me-code">{t('Code reçu avec !me')}</label>
        <input id="me-code" bind:value={code} maxlength="7" autocomplete="off" autocapitalize="characters" spellcheck="false" placeholder={t('Code !me')} />
        <button class="btn" disabled={claiming || !code.trim()}>{t('Valider')}</button>
      </form>
    </div>
  </div>
{/if}

<Seg label={t('Période')} value={period} onchange={v => (period = v)} items={[['7d', t('7 jours')], ['30d', t('30 jours')], ['90d', t('90 jours')]]} />

<label class="search">
  <Icon name="search" />
  <span class="sr-only">{t('Rechercher un joueur')}</span>
  <input type="search" bind:value={query} placeholder={priv ? t('Pseudo, ancien pseudo, IP…') : t('Pseudo, ancien pseudo…')} autocomplete="off" spellcheck="false" />
</label>

<div class="chips" role="group" aria-label={t('Filtrer les joueurs')}>
  <button aria-pressed={filter === 'all'} onclick={() => (filter = 'all')}>{t('Tous')}</button>
  <button aria-pressed={filter === 'online'} onclick={() => (filter = 'online')}>{t('En ligne ({n})', { n: online.length })}</button>
  <button aria-pressed={filter === 'new'} onclick={() => (filter = 'new')}>{t('Nouveaux ({n})', { n: newKeys.size })}</button>
  {#if priv}<button aria-pressed={filter === 'note'} onclick={() => (filter = 'note')}>{t('Notés')}</button>{/if}
  <span class="muted period">{periodText}</span>
</div>

{#if list === null}
  <p class="muted">{t('Chargement…')}</p>
{:else if failed}
  <p class="muted">{t('Liste des joueurs indisponible.')}</p>
{:else}
  {#snippet who(p)}
    <a class="tn" href={href('joueur', p.key)}>
      <span class="dot2" class:on={p.online}></span><CodName raw={p.name_raw} />
      {#if newKeys.has(p.key)}<span class="new">{t('Nouveau')}</span>{/if}
      {#if noteOf(p)}<span class="nt" title={noteOf(p).note}>{noteOf(p).tag ? tagLabel(noteOf(p).tag) : t('Note')}</span>{/if}
    </a>
    {#if others(p).length}<span class="aka">{t('aussi : {names}', { names: others(p).join(', ') })}</span>{/if}
  {/snippet}
  <section class="card table">
    <DataTable rows={rows.slice(0, shown)} cols={COLS} name={who} rank={false} keyOf={p => p.key} mine={me?.key ?? ''} label={t('Joueurs')}
      empty={query ? t('Aucun joueur ne correspond.') : t('Aucun joueur sur la période')} />
  </section>
  <div class="list card">
    {#each rows.slice(0, shown) as p (p.key)}
      <a class="row" href={href('joueur', p.key)} class:mine={p.key === me?.key}>
        <span class="av">{(p.name_clean || '?').charAt(0).toUpperCase()}<span class="dot" class:on={p.online}></span></span>
        <span class="mid">
          <span class="pn"><CodName raw={p.name_raw} />{#if p.key === me?.key}<span class="you">{t('toi')}</span>{/if}{#if newKeys.has(p.key)}<span class="new">{t('Nouveau')}</span>{/if}{#if noteOf(p)}<span class="nt" title={noteOf(p).note}>{noteOf(p).tag ? tagLabel(noteOf(p).tag) : t('Note')}</span>{/if}</span>
          <span class="sub">
            {#if p.online}{t('En ligne')}{#if srv?.map}{' · '}{mapName(srv.map)}{/if}{:else}{t('Vu {when}', { when: agoText(p.last_seen) })}{/if}{#if p.level}{' · '}{t('Niv. {n}', { n: p.level })}{/if}
          </span>
          <span class="sub">{#if p.source === 'b3'}{t('Vu par B3')}{#if p.sessions}{' · '}{t('{n} connexions', { n: p.sessions })}{/if}{:else if p.source === 'log'}{t('Vu dans le log du jeu')}{:else}{dur(p.playtime_s)} {t('de jeu')} · {t('{n} sessions', { n: p.sessions ?? 0 })}{/if}{#if pingOf(p) != null}{' · '}<span style:color={pingColor(pingOf(p))}>{Math.round(pingOf(p))} ms</span>{/if}{#if priv && p.ip}{' · '}{p.ip}{/if}</span>
        </span>
        <span class="kd"><b>{kdOf(p) == null ? '—' : kdOf(p).toFixed(2)}</b><small>K/D</small></span>
      </a>
    {:else}
      <p class="empty">{query ? t('Aucun joueur ne correspond.') : filter === 'online' ? t('Personne en jeu pour le moment.') : t('Aucun joueur vu ces 90 derniers jours.')}</p>
    {/each}
  </div>
  {#if rows.length > shown}
    <button class="btn more" onclick={() => (shown += PAGE)}>{t('Afficher plus ({n} restants)', { n: rows.length - shown })}</button>
  {/if}
{/if}

<style>
  .title { margin-bottom: 14px; }
  .me { display: flex; flex-direction: column; gap: 12px; padding: 14px 16px; margin-bottom: 14px; text-decoration: none; color: var(--text); border-color: var(--accent); background: var(--accent-soft); }
  .top { display: flex; align-items: center; gap: 12px; }
  .me .av { width: 44px; height: 44px; border-radius: 14px; background: var(--accent); color: var(--on-accent); font-size: 18px; }
  .who { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .nm { font-family: var(--font-display); font-size: 22px; font-weight: 700; display: flex; min-width: 0; }
  .kpis { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .kpis span { display: flex; flex-direction: column; }
  .kpis b { font-family: var(--font-display); font-size: 18px; }
  .kpis small { font-size: 12px; color: var(--text-2); }

  .hint { display: flex; gap: 12px; align-items: flex-start; padding: 14px 16px; margin-bottom: 14px; border-radius: var(--radius); border: 1px dashed #33405A; background: #11161E; }
  .ic { width: 36px; height: 36px; flex: none; border-radius: 11px; background: var(--accent-soft); color: var(--accent); display: flex; align-items: center; justify-content: center; }
  .txt { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .txt > span { font-size: 13px; color: var(--text-2); }
  .code { display: flex; gap: 8px; margin-top: 10px; }
  .code input { flex: 1; min-width: 0; max-width: 160px; height: 42px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; font-weight: 800; letter-spacing: .12em; text-transform: uppercase; }
  .code .btn { min-height: 42px; }

  .search { display: flex; align-items: center; gap: 10px; height: 48px; padding: 0 14px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface); color: var(--muted); }
  .search input { flex: 1; min-width: 0; height: 44px; border: 0; background: transparent; font-size: 16px; color: var(--text); outline: none; }
  .search:focus-within { border-color: var(--accent); }
  .chips { display: flex; align-items: center; gap: 8px; margin: 12px 0; flex-wrap: wrap; }
  .chips button { height: 36px; padding: 0 14px; border-radius: 18px; border: 1px solid var(--line); background: transparent; color: var(--text-2); font-weight: 700; font-size: 14px; }
  .chips button[aria-pressed='true'] { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .period { font-size: 12px; margin-left: auto; }

  .list { overflow: hidden; }
  .row { display: flex; align-items: center; gap: 12px; min-height: 64px; padding: 8px 14px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); }
  .row:first-child { border-top: 0; }
  .row.mine { background: var(--accent-soft); }
  .av { position: relative; width: 40px; height: 40px; flex: none; border-radius: 12px; background: var(--surface-2); display: flex; align-items: center; justify-content: center; font-weight: 800; }
  .dot { position: absolute; right: -2px; bottom: -2px; width: 11px; height: 11px; border-radius: 6px; border: 2px solid var(--surface); background: #4A5568; }
  .dot.on { background: var(--ok); }
  .mid { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .pn { display: flex; align-items: center; gap: 6px; min-width: 0; font-weight: 700; }
  .you { flex: none; font-size: 11px; font-weight: 800; color: var(--on-accent); background: var(--accent); border-radius: 6px; padding: 1px 6px; }
  .sub { font-size: 13px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .kd { display: flex; flex-direction: column; align-items: flex-end; }
  .kd b { font-family: var(--font-display); font-size: 17px; font-variant-numeric: tabular-nums; }
  .kd small { font-size: 11px; color: var(--muted); }
  .empty { margin: 0; padding: 18px 16px; color: var(--muted); }
  .more { width: 100%; margin-top: 12px; }
  .new, .nt { flex: none; font-size: 10px; font-weight: 800; border-radius: 6px; padding: 1px 6px; }
  .new { color: var(--accent); background: var(--accent-soft); }
  .nt { color: var(--warn); border: 1px solid var(--warn); max-width: 120px; overflow: hidden; text-overflow: ellipsis; }
  .table { display: none; padding: 10px 12px; }
  .tn { display: flex; align-items: center; gap: 6px; min-width: 0; color: var(--text); text-decoration: none; }
  .dot2 { flex: none; width: 8px; height: 8px; border-radius: 4px; background: #4A5568; }
  .dot2.on { background: var(--ok); }
  .aka { display: block; font-size: 11px; font-weight: 600; color: var(--muted); max-width: 240px; overflow: hidden; text-overflow: ellipsis; }
  .search { margin-top: 12px; }

  @media (min-width: 960px) {
    .list { display: none; }
    .table { display: block; }
  }
</style>
