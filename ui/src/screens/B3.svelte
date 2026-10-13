<script>
  // Outils B3 (private data, when the server has a B3 database): sanctions (filters, lift one), admins and their grade,
  // player search like !lookup, and moderation statistics over 90 days. Actions are for the owner.
  import BackTitle from '../components/BackTitle.svelte';
  import Seg from '../components/Seg.svelte';
  import CodName from '../components/CodName.svelte';
  import { get, post } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf, isOwner } from '../lib/perms.js';
  import { href, go } from '../lib/router.svelte.js';
  import { clean } from '../lib/format.js';
  import { dt, day, PEN_LABEL, PEN_COLOR, penEnd, penBy } from '../lib/moderation.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const srv = $derived(currentServer());
  const allowed = $derived(permsOf(app.me, id).has('private'));
  const owner = $derived(isOwner(app.me, id));

  let tab = $state('pen');
  let on = $state(null); // null loading, false no B3, true
  let offMsg = $state('');
  let groups = $state([]);
  let stats = $state(null);

  $effect(() => {
    const sid = id;
    if (!sid || !allowed) return;
    let alive = true;
    on = null;
    stats = null;
    get(`/api/servers/${sid}/b3`).then(r => {
      if (!alive) return;
      groups = r.groups ?? [];
      on = true;
      loadStats(sid);
    }).catch(e => { if (alive) { on = false; offMsg = t(e.message); } });
    return () => { alive = false; };
  });
  const loadStats = sid => get(`/api/servers/${sid}/b3/stats?days=90`).then(r => { stats = r; }).catch(() => { stats = {}; });

  // --- sanctions
  const TYPES = [['', 'Toutes'], ['Ban', 'Bans'], ['TempBan', 'Bans temporaires'], ['Kick', 'Kicks'], ['Warning', 'Avertissements']];
  let type = $state('');
  let activeOnly = $state(false);
  let q = $state('');
  let dq = $state('');
  let pens = $state(null);
  let sure = $state('');
  let busy = $state('');
  let qt;
  const typedQ = () => { clearTimeout(qt); qt = setTimeout(() => (dq = q.trim()), 300); };

  function loadPens() {
    const p = new URLSearchParams({ type, active: activeOnly ? '1' : '', q: dq, limit: '500' });
    return get(`/api/servers/${id}/b3/penalties?${p}`).then(r => { pens = r.penalties ?? []; }).catch(() => { pens = []; });
  }
  $effect(() => {
    type; activeOnly; dq;
    if (on && tab === 'pen') loadPens();
  });

  async function liftPen(x) {
    const k = String(x.id);
    if (sure !== k) { sure = k; return; }
    busy = k;
    try {
      if (x.type === 'Ban' || x.type === 'TempBan') {
        await post(`/api/servers/${id}/b3/unban`, { guid: x.guid, name: x.name });
        say(t('Ban levé : {name}', { name: clean(x.name) }));
      } else {
        await post(`/api/servers/${id}/b3/penalty/${x.id}/lift`, {});
        say(t('Sanction retirée'));
      }
      await loadPens();
      loadStats(id);
    } catch (e) {
      say(t('Échec : {err}', { err: t(e.message) }));
    } finally {
      busy = '';
      sure = '';
    }
  }

  // --- admins
  let admins = $state(null);
  let pending = $state(null); // {guid, name, group}
  const loadAdmins = () => get(`/api/servers/${id}/b3/admins`).then(r => { admins = r.admins ?? []; if (r.groups) groups = r.groups; }).catch(() => { admins = []; });
  $effect(() => { if (on && tab === 'admins') loadAdmins(); });
  const groupOf = gid => groups.find(g => g.id === gid);

  async function setGrade() {
    const p = pending;
    busy = 'grade';
    try {
      const r = await post(`/api/servers/${id}/b3/group`, { guid: p.guid, name: p.name, group: p.group });
      say(t('{name} : grade {g}', { name: clean(p.name), g: r.group?.name ?? '' }) + (r.online ? ` (${t('actif à sa prochaine connexion')})` : ''));
      pending = null;
      loadAdmins();
    } catch (e) {
      say(t('Échec : {err}', { err: t(e.message) }));
    } finally {
      busy = '';
    }
  }

  // --- search
  let find = $state('');
  let found = $state(null);
  let foundErr = $state('');
  let ft;
  function typedFind() {
    clearTimeout(ft);
    ft = setTimeout(() => {
      const v = find.trim();
      if (!v) { found = null; return; }
      get(`/api/servers/${id}/b3/search?q=${encodeURIComponent(v)}`)
        .then(r => { found = r.clients ?? []; foundErr = ''; })
        .catch(e => { found = []; foundErr = t(e.message); });
    }, 300);
  }

  // --- stats
  const tot = $derived(stats?.totals ?? {});
  const orig = $derived(stats?.origins ?? {});
  const weeks = $derived(stats?.weeks ?? []);
  const KINDS = ['Ban', 'TempBan', 'Kick', 'Warning'];
  const weekMax = $derived(Math.max(1, ...weeks.map(w => KINDS.reduce((s, k) => s + (w.types?.[k] ?? 0), 0))));
  const kpis = $derived([
    [stats?.active_bans ?? 0, 'bans en cours'], [(tot.Ban ?? 0) + (tot.TempBan ?? 0), 'bans (90 j)'], [tot.Kick ?? 0, 'kicks (90 j)'], [tot.Warning ?? 0, 'avertissements (90 j)']
  ]);
</script>

<BackTitle title={t('Outils B3')} sub={srv ? clean(srv.name) : ''} fallback="moderation" />

{#if !allowed}
  <p class="muted">{t('Il faut l\'accès aux données privées de ce serveur.')}</p>
{:else if on === null}
  <p class="muted">{t('Chargement…')}</p>
{:else if !on}
  <p class="muted">{offMsg || t('B3 n\'est pas configuré pour ce serveur.')}</p>
{:else}
  <div class="tabs"><Seg label={t('Vue')} value={tab} onchange={v => (tab = v)} items={[['pen', t('Sanctions')], ['admins', t('Admins')], ['find', t('Recherche')], ['stats', t('Stats')]]} /></div>

  {#if tab === 'pen' || tab === 'stats'}
    <div class="kpis">
      {#each kpis as [n, l] (l)}<div class="card kpi"><b>{n}</b><span>{t(l)}</span></div>{/each}
    </div>
    <p class="muted small orig">{t('Origine (90 j) : {a} automatique(s) par B3 · {g} par un admin en jeu · {s} depuis le site', { a: orig.auto ?? 0, g: orig.admin ?? 0, s: orig.site ?? 0 })}</p>
  {/if}

  {#if tab === 'pen'}
    <div class="chips" role="group" aria-label={t('Type de sanction')}>
      {#each TYPES as [k, l] (k)}<button aria-pressed={type === k} onclick={() => (type = k)}>{t(l)}</button>{/each}
    </div>
    <div class="bar">
      <label class="sr-only" for="b3-q">{t('Rechercher')}</label>
      <input id="b3-q" bind:value={q} oninput={typedQ} placeholder={t('Joueur, GUID ou raison…')} autocomplete="off" autocapitalize="none" spellcheck="false" />
      <label class="check"><input type="checkbox" bind:checked={activeOnly} />{t('en cours seulement')}</label>
    </div>
    {#if pens === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else}
      <div class="card rows">
        {#each pens as x (x.id)}
          <div class="pen">
            <div class="ph">
              <span class="kind" class:off={!x.active && x.type !== 'Kick' && x.type !== 'Notice'} style:--c={PEN_COLOR[x.type] ?? 'var(--text-2)'}>{t(PEN_LABEL[x.type] ?? x.type)}</span>
              {#if x.guid}<a href={href('joueur', x.guid)}><CodName raw={x.name} /></a>{:else}<CodName raw={x.name} />{/if}
              <span class="muted when">{dt(x.time_add)}</span>
            </div>
            <p class="why">{x.reason || '—'}{#if x.keyword}<span class="kw">#{x.keyword}</span>{/if}</p>
            <div class="pf">
              <span class="muted">{t('par {who}', { who: penBy(x) })}{#if x.type !== 'Kick' && x.type !== 'Notice'}{' · '}{t('fin : {d}', { d: penEnd(x) })}{/if}</span>
              {#if owner && x.active}
                <button class="link" class:danger={sure === String(x.id)} disabled={busy === String(x.id)} onclick={() => liftPen(x)}>
                  {sure === String(x.id) ? t('Confirmer') : x.type === 'Ban' || x.type === 'TempBan' ? t('Lever') : t('Retirer')}
                </button>
              {/if}
            </div>
          </div>
        {:else}
          <p class="muted empty">{t('Aucune sanction.')}</p>
        {/each}
      </div>
    {/if}
  {:else if tab === 'admins'}
    <p class="muted small intro">{t('Grade : le niveau B3 (20 modo … 100 super admin). Il s\'applique à la prochaine connexion du joueur.')}</p>
    {#if admins === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else}
      <div class="card rows">
        {#each admins as a (a.guid)}
          <div class="adm">
            <span class="who">
              <a href={href('joueur', a.guid)}><CodName raw={a.name} /></a>
              <small class="muted">{t('{n} sanctions données', { n: a.given })} · {t('vu le {d}', { d: day(a.time_edit) })}{#if a.mask}{' · '}{t('affiché comme {g}', { g: a.mask.name })}{/if}</small>
            </span>
            {#if owner}
              <select aria-label={t('Grade de {name}', { name: clean(a.name) })} value={pending?.guid === a.guid ? pending.group : a.group.id}
                onchange={e => (pending = { guid: a.guid, name: a.name, group: Number(e.currentTarget.value) })}>
                {#each groups as g (g.id)}<option value={g.id}>{g.name} · {g.level}</option>{/each}
              </select>
            {:else}
              <span class="grade">{a.group?.name ?? 'Guest'} · {a.group?.level ?? 0}</span>
            {/if}
            {#if pending?.guid === a.guid && pending.group !== a.group.id}
              <span class="confirm">
                <span>{t('Passer au grade {g} ({l}) ?', { g: groupOf(pending.group)?.name ?? '', l: groupOf(pending.group)?.level ?? '?' })}</span>
                <button class="btn primary" disabled={busy === 'grade'} onclick={setGrade}>{t('Confirmer')}</button>
                <button class="btn ghost" onclick={() => (pending = null)}>{t('Annuler')}</button>
              </span>
            {/if}
          </div>
        {:else}
          <p class="muted empty">{t('Aucun admin — tape !iamgod en jeu pour devenir super admin')}</p>
        {/each}
      </div>
    {/if}
  {:else if tab === 'find'}
    <label class="sr-only" for="b3-find">{t('Rechercher')}</label>
    <input id="b3-find" class="filter" bind:value={find} oninput={typedFind} placeholder={t('Pseudo, IP (ex : 82.64.) ou GUID…')} autocomplete="off" autocapitalize="none" spellcheck="false" />
    <p class="muted small intro">{t('Recherche par pseudo (actuel ou ancien), IP ou GUID, comme !lookup.')}</p>
    {#if found}
      <div class="card rows">
        {#each found as c (c.guid)}
          <button class="hit" onclick={() => go('joueur', c.guid)}>
            <span class="ph"><CodName raw={c.name} />{#if c.banned}<span class="kind" style:--c={PEN_COLOR.Ban}>{t('banni')}</span>{/if}<span class="grade">{c.group?.name ?? 'Guest'} · {c.group?.level ?? 0}</span></span>
            <small class="muted">{t('{n} connexions', { n: c.connections })} · {t('du {a} au {b}', { a: day(c.time_add), b: day(c.time_edit) })}{#if c.ip}{' · '}{c.ip}{/if}</small>
          </button>
        {:else}
          <p class="muted empty">{foundErr || t('Aucun joueur trouvé')}</p>
        {/each}
      </div>
    {/if}
  {:else}
    <section class="card box">
      <h2 class="label">{t('Sanctions par semaine')}</h2>
      {#if !weeks.length}
        <p class="muted">{t('Aucune sanction.')}</p>
      {:else}
        <div class="chart" role="img" aria-label={t('Sanctions par semaine')}>
          {#each weeks as w (w.start)}
            {@const n = KINDS.reduce((s, k) => s + (w.types?.[k] ?? 0), 0)}
            <div class="col" title={`${t('Semaine du {d}', { d: day(w.start) })} : ${n}`}>
              <span class="n">{n || ''}</span>
              <div class="stack" style:height={`calc((100% - 30px) * ${n / weekMax})`}>
                {#each KINDS as k (k)}{#if w.types?.[k]}<span style:flex-grow={w.types[k]} style:background={PEN_COLOR[k]}></span>{/if}{/each}
              </div>
              <span class="d">{day(w.start).slice(0, 5)}</span>
            </div>
          {/each}
        </div>
        <div class="legend">{#each KINDS as k (k)}<span><i style:background={PEN_COLOR[k]}></i>{t(PEN_LABEL[k])}</span>{/each}</div>
      {/if}
    </section>
    <div class="tops">
      {#each [['Raisons les plus fréquentes', stats?.reasons, false], ['Joueurs les plus sanctionnés', stats?.players, true], ['Sanctions données par', stats?.admins, false]] as [title, list, coded] (title)}
        <section class="card box">
          <h2 class="label">{t(title)}</h2>
          {#each list ?? [] as x, i (i)}
            <div class="top"><span class="rank">{i + 1}</span><span class="tk">{#if coded}<CodName raw={x.key} />{:else}{x.key || '—'}{/if}</span><b>{x.n}</b></div>
          {:else}
            <p class="muted">—</p>
          {/each}
        </section>
      {/each}
    </div>
  {/if}
{/if}

<style>
  .tabs { margin-bottom: 12px; max-width: 520px; }
  .kpis { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .kpi { padding: 10px 14px; display: flex; flex-direction: column; }
  .kpi b { font-family: var(--font-display); font-size: 22px; }
  .kpi span { font-size: 12px; color: var(--muted); }
  .small { font-size: 12px; }
  .orig { margin: 8px 2px 12px; }
  .intro { margin: 0 2px 10px; }
  .chips { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 2px; scrollbar-width: none; }
  .chips button { flex: none; height: 36px; padding: 0 12px; border-radius: 18px; border: 1px solid var(--line); background: transparent; color: var(--text-2); font-weight: 700; font-size: 13px; }
  .chips button[aria-pressed='true'] { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .bar { display: flex; flex-wrap: wrap; gap: 8px 14px; align-items: center; margin: 10px 0 12px; }
  .bar input:not([type]) { flex: 1 1 220px; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); padding: 0 12px; font-size: 16px; }
  .filter { width: 100%; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); padding: 0 12px; font-size: 16px; margin-bottom: 8px; }
  .check { display: inline-flex; align-items: center; gap: 8px; font-size: 14px; color: var(--text-2); font-weight: 700; }
  .check input { width: 18px; height: 18px; accent-color: var(--accent); }
  .rows { overflow: hidden; }
  .empty { margin: 0; padding: 14px 16px; }
  .pen { padding: 10px 16px; border-top: 1px solid var(--line-soft); display: flex; flex-direction: column; gap: 3px; min-width: 0; }
  .pen:first-child, .adm:first-child, .hit:first-child { border-top: 0; }
  .ph { display: flex; align-items: center; gap: 8px; min-width: 0; font-weight: 700; }
  .ph a { color: var(--text); text-decoration: none; display: inline-flex; min-width: 0; }
  .when { margin-left: auto; font-size: 12px; font-weight: 500; white-space: nowrap; }
  .kind { flex: none; font-size: 11px; font-weight: 800; padding: 2px 7px; border-radius: 7px; color: var(--c); background: color-mix(in srgb, var(--c) 16%, transparent); }
  .kind.off { opacity: .55; text-decoration: line-through; }
  .why { margin: 0; font-size: 14px; color: var(--text-2); overflow-wrap: anywhere; }
  .kw { margin-left: 6px; font-size: 12px; color: var(--muted); }
  .pf { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 12px; }
  .link { border: 0; background: none; color: var(--accent); font-weight: 800; font-size: 13px; padding: 6px 0; flex: none; }
  .link.danger { color: var(--danger-text); }
  .adm { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 8px 12px; padding: 10px 16px; border-top: 1px solid var(--line-soft); }
  .who { display: flex; flex-direction: column; gap: 2px; min-width: 0; flex: 1 1 220px; font-weight: 700; }
  .who a { color: var(--text); text-decoration: none; display: inline-flex; min-width: 0; }
  .who small { font-size: 12px; font-weight: 500; }
  select { height: 40px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 8px; font-size: 14px; max-width: 100%; }
  .grade { flex: none; font-size: 12px; font-weight: 800; padding: 3px 8px; border-radius: 8px; background: var(--surface-2); color: var(--text-2); }
  .confirm { flex: 1 1 100%; display: flex; flex-wrap: wrap; align-items: center; gap: 8px; font-size: 13px; }
  .confirm .btn { min-height: 38px; padding: 0 12px; font-size: 13px; }
  .hit { width: 100%; display: flex; flex-direction: column; gap: 3px; padding: 10px 16px; border: 0; border-top: 1px solid var(--line-soft); background: transparent; color: var(--text); text-align: left; }
  .hit .grade { margin-left: auto; }
  .hit small { font-size: 12px; overflow-wrap: anywhere; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .box p { margin: 0; }
  .chart { display: flex; align-items: stretch; gap: 3px; height: 180px; padding-top: 4px; }
  .col { flex: 1; min-width: 0; display: flex; flex-direction: column; justify-content: flex-end; align-items: center; gap: 3px; }
  .col .n { font-size: 10px; color: var(--muted); }
  .stack { width: 100%; max-width: 34px; display: flex; flex-direction: column-reverse; border-radius: 4px 4px 0 0; overflow: hidden; min-height: 0; }
  .stack span { display: block; flex-basis: 0; }
  .col .d { font-size: 9px; color: var(--muted); white-space: nowrap; }
  .legend { display: flex; flex-wrap: wrap; gap: 6px 14px; font-size: 12px; color: var(--text-2); }
  .legend span { display: inline-flex; align-items: center; gap: 6px; }
  .legend i { width: 10px; height: 10px; border-radius: 3px; }
  .tops { display: grid; gap: 12px; margin-top: 12px; }
  .top { display: flex; align-items: center; gap: 10px; font-size: 14px; min-width: 0; }
  .rank { width: 18px; color: var(--muted); font-size: 12px; flex: none; }
  .tk { flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; display: inline-flex; }
  @media (max-width: 520px) { .col .d { display: none; } }
  @media (min-width: 760px) {
    .kpis { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .tops { grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: start; }
  }
</style>
