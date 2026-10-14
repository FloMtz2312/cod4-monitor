<script>
  // A player's profile: header (online, level), "C'est moi", Discord card, key figures, then
  // Résumé (form, hours, nemesis), Combat (weapons, zones, sniper figures), Matchs, and Modération for the staff.
  import Icon from '../components/Icon.svelte';
  import CodName from '../components/CodName.svelte';
  import PlayerSummary from '../components/player/PlayerSummary.svelte';
  import PlayerCombat from '../components/player/PlayerCombat.svelte';
  import PlayerMatches from '../components/player/PlayerMatches.svelte';
  import PlayerModeration from '../components/player/PlayerModeration.svelte';
  import { get } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { meOn, setMe, clearMe } from '../lib/me.svelte.js';
  import { route, back, go, href } from '../lib/router.svelte.js';
  import { shareCard } from '../lib/card.js';
  import { clean, agoText, fmt } from '../lib/format.js';
  import { dur, day } from '../lib/dates.js';
  import { t } from '../lib/i18n.svelte.js';
  import { say } from '../lib/toast.svelte.js';

  const key = $derived(route.params[0] ?? '');
  const id = $derived(app.serverId);
  const srv = $derived(currentServer());
  const perms = $derived(permsOf(app.me, id));
  const isMe = $derived(meOn(id)?.key === key);

  let p = $state(null);
  let failed = $state('');
  let form = $state(null);
  let matches = $state(null);
  let progress = $state(null);
  let live = $state([]);
  let tab = $state('summary');

  $effect(() => {
    const sid = id, k = key;
    if (!sid || !k) return;
    let alive = true;
    p = form = matches = progress = null;
    failed = '';
    live = [];
    tab = 'summary';
    const base = `/api/servers/${sid}`, q = encodeURIComponent(k);
    get(`${base}/player?key=${q}`).then(r => { if (alive) p = r; }).catch(e => { if (alive) failed = e.message; });
    get(`${base}/form?key=${q}`).then(r => { if (alive) form = r; }).catch(() => {});
    get(`${base}/matches?key=${q}&range=30d`).then(r => { if (alive) matches = r.matches ?? []; }).catch(() => { if (alive) matches = []; });
    get(`${base}/progress?key=${q}`).then(r => { if (alive) progress = r.weeks ?? []; }).catch(() => {});
    get(`${base}/status`).then(r => { if (alive) live = r.players ?? []; }).catch(() => {});
    return () => { alive = false; };
  });

  const one = v => (Array.isArray(v) ? v[0] : v) ?? {};
  const names = $derived(p?.names ?? []);
  const nameRaw = $derived(p?.name_raw || names[0]?.name_raw || (meOn(id)?.key === key ? meOn(id).name_raw : '') || (key.startsWith('n:') ? key.slice(2) : ''));
  const combat = $derived(one(p?.combat));
  const sniper = $derived(one(p?.sniper));
  const summary = $derived(one(p?.summary));
  const acc = $derived(one(p?.accuracy));
  // online now: one of the player's names is in the last status of the server
  const livePlayer = $derived.by(() => {
    const all = new Set(names.map(n => n.name_clean));
    if (!all.size && nameRaw) all.add(clean(nameRaw));
    const x = live.find(l => all.has(l.name_clean));
    return x ? { ...x, key } : null;
  });
  const lastSeen = $derived(summary.last_seen ?? names[0]?.last_seen ?? null);
  // private data in the answer (staff): the GUID line and the other names with their dates
  const priv = $derived(!!p && p.sessions !== undefined);

  const kpis = $derived.by(() => {
    if (!p) return [];
    const k = combat.kills ?? 0, d = combat.deaths ?? 0;
    const out = [
      { label: t('K/D'), v: (k / Math.max(1, d)).toFixed(2) },
      { label: t('Kills'), v: fmt(k) },
      { label: t('Headshots'), v: k ? `${Math.round((100 * (combat.headshots ?? 0)) / k)} %` : '—' }
    ];
    if (sniper.longest != null) out.push({ label: t('Plus long kill'), v: `${Math.round(sniper.longest)} m` });
    else if (acc.shots) out.push({ label: t('Précision'), v: `${Math.round((100 * acc.hits) / acc.shots)} %` });
    else out.push({ label: t('Morts'), v: fmt(d) });
    out.push(
      { label: t('Temps de jeu'), v: summary.playtime_s ? dur(summary.playtime_s) : '—' },
      { label: t('Sessions'), v: fmt(summary.sessions ?? 0) },
      { label: t('Première apparition'), v: day(summary.first_seen), small: true },
      { label: t('Dégâts / round'), v: acc.rounds ? fmt(Math.round((acc.dmg_done ?? 0) / acc.rounds)) : '—' }
    );
    return out;
  });

  const tabs = $derived([
    ['summary', t('Résumé')],
    ['combat', t('Combat')],
    ['matches', t('Matchs')],
    ...(perms.has('private') || perms.has('notes') ? [['mod', t('Modération')]] : [])
  ]);

  async function copyLink() {
    const url = location.origin + '/' + href('joueur', key);
    try {
      await navigator.clipboard.writeText(url);
      say(t('Lien copié'));
    } catch {
      say(url);
    }
  }

  function toggleMe() {
    if (isMe) {
      clearMe(id);
      say(t('Fiche désépinglée'));
    } else {
      setMe(id, key, nameRaw);
      say(t('Tes stats sont épinglées dans Joueurs'));
    }
  }
</script>

<button class="back" onclick={() => back('joueurs')}><Icon name="back" />{t('Retour')}</button>

{#if failed}
  <section class="card pad"><p>{failed}</p></section>
{:else if !p}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <header class="head">
    <span class="av">{clean(nameRaw).charAt(0).toUpperCase() || '?'}</span>
    <div class="id">
      <h1 class="nm"><CodName raw={nameRaw || key} /></h1>
      <div class="meta">
        {#if livePlayer}
          <span class="st on"><span class="dot"></span>{t('En ligne')}{#if srv}{' · '}{clean(srv.name)}{/if}</span>
        {:else if lastSeen}
          <span class="st"><span class="dot"></span>{t('Vu {when}', { when: agoText(lastSeen) })}</span>
        {/if}
        {#if sniper.level}<span class="lv">{t('Niv. {n}', { n: sniper.level })}{#if sniper.prestige}{' · '}{t('Prestige {n}', { n: sniper.prestige })}{/if}</span>{/if}
      </div>
    </div>
  </header>

  {#if priv}
    <div class="ids">
      <span class="mono">{key.startsWith('n:') ? t('Pas de GUID connu (identifié par son pseudo)') : `GUID ${key}`}</span>
      {#if names.length > 1}
        <span class="also">{t('aussi :')}
          {#each names.slice(1) as n, i (n.name_clean)}<span class="alias" title={t('vu du {a} au {b}', { a: day(n.first_seen), b: day(n.last_seen) })}><CodName raw={n.name_raw} /><small>{day(n.first_seen)} → {day(n.last_seen)}</small></span>{#if i < names.length - 2}{' '}{/if}{/each}
        </span>
      {/if}
    </div>
  {/if}

  <div class="btns">
    <button class="btn me" class:on={isMe} aria-pressed={isMe} onclick={toggleMe}>
      <Icon name={isMe ? 'check' : 'star'} size={18} />{t('C\'est moi')}
    </button>
    <button class="btn" onclick={() => shareCard(id, 'player', key)}><Icon name="image" size={18} />{t('Carte Discord')}</button>
    <button class="btn" onclick={copyLink}><Icon name="link" size={18} />{t('Copier le lien')}</button>
    <button class="btn" onclick={() => go('duel', key)}><Icon name="users" size={18} />{t('Duel')}</button>
  </div>

  <div class="kpis">
    {#each kpis as k (k.label)}<div class="card"><span>{k.label}</span><b class:small={k.small}>{k.v}</b></div>{/each}
  </div>

  <div class="seg" role="tablist" aria-label={t('Sections de la fiche')}>
    {#each tabs as [k, label] (k)}
      <button role="tab" aria-selected={tab === k} class:on={tab === k} onclick={() => (tab = k)}>{label}</button>
    {/each}
  </div>

  {#if tab === 'summary'}
    <PlayerSummary {p} {form} {matches} {progress} playerKey={key} serverId={id} />
  {:else if tab === 'combat'}
    <PlayerCombat {p} />
  {:else if tab === 'matches'}
    <PlayerMatches {matches} />
  {:else if tab === 'mod'}
    <PlayerModeration {p} {key} {nameRaw} serverId={id} {perms} {livePlayer} />
  {/if}
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 4px; height: 40px; padding: 0 10px 0 4px; margin: -4px 0 8px -4px; border: 0; background: transparent; color: var(--text-2); font-weight: 700; }
  .pad { padding: 16px; }
  .head { display: flex; align-items: center; gap: 14px; }
  .av { width: 64px; height: 64px; flex: none; border-radius: 20px; background: var(--surface-2); border: 1px solid var(--line); display: flex; align-items: center; justify-content: center; font-family: var(--font-display); font-size: 28px; font-weight: 700; }
  .id { min-width: 0; display: flex; flex-direction: column; gap: 6px; }
  .nm { font-family: var(--font-display); font-size: 28px; font-weight: 700; line-height: 1.1; display: flex; min-width: 0; overflow-wrap: anywhere; }
  .meta { display: flex; flex-wrap: wrap; gap: 6px; font-size: 13px; }
  .st, .lv { display: inline-flex; align-items: center; gap: 6px; height: 26px; padding: 0 10px; border-radius: 13px; background: var(--surface-2); color: var(--text-2); font-weight: 700; }
  .st .dot { width: 7px; height: 7px; border-radius: 4px; background: #4A5568; }
  .st.on { color: var(--ok); background: rgba(61, 214, 140, .12); }
  .st.on .dot { background: var(--ok); }
  .btns { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; margin: 16px 0 12px; }
  .btns .btn { min-width: 0; padding: 0 10px; }
  .btn.me { border-color: var(--accent); color: var(--accent); background: transparent; }
  .btn.me.on { background: var(--accent); color: var(--on-accent); }
  .kpis { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; margin-bottom: 14px; }
  .kpis .card { padding: 12px 14px; display: flex; flex-direction: column; gap: 2px; }
  .kpis span { font-size: 12px; color: var(--muted); font-weight: 700; }
  .kpis b.small { font-size: 17px; line-height: 1.6; }
  .ids { display: flex; flex-direction: column; gap: 6px; margin-top: 12px; font-size: 12px; color: var(--muted); }
  .mono { font-family: ui-monospace, monospace; overflow-wrap: anywhere; }
  .also { display: flex; flex-wrap: wrap; align-items: center; gap: 6px; }
  .alias { display: inline-flex; align-items: baseline; gap: 6px; max-width: 100%; padding: 3px 8px; border-radius: 8px; background: var(--surface-2); color: var(--text); font-size: 13px; font-weight: 700; }
  .alias small { font-size: 11px; font-weight: 600; color: var(--muted); white-space: nowrap; }
  .kpis b { font-family: var(--font-display); font-size: 24px; font-variant-numeric: tabular-nums; }
  .seg { display: flex; gap: 4px; padding: 4px; margin-bottom: 14px; background: var(--surface); border: 1px solid var(--line); border-radius: 14px; }
  .seg button { flex: 1 1 auto; min-width: 0; height: 38px; padding: 0 6px; border: 0; border-radius: 10px; background: transparent; color: var(--text-2); font-weight: 700; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .seg button.on { background: var(--accent); color: var(--on-accent); }
  @media (min-width: 720px) {
    .kpis { grid-template-columns: repeat(4, minmax(0, 1fr)); }
    .btns { display: flex; flex-wrap: wrap; }
  }
</style>
