<script>
  // The live screen: the game in progress (map, score, players by team, match feed, chat) and quick actions for
  // the staff. Never empty: when nobody plays, the last 24 hours and the week of the server.
  import { untrack } from 'svelte';
  import Icon from '../components/Icon.svelte';
  import LivePlayers from '../components/LivePlayers.svelte';
  import MatchFeed from '../components/MatchFeed.svelte';
  import LiveChat from '../components/LiveChat.svelte';
  import WeekCard from '../components/WeekCard.svelte';
  import TodayCard from '../components/TodayCard.svelte';
  import PlayerSheet from '../components/PlayerSheet.svelte';
  import MapSheet from '../components/MapSheet.svelte';
  import RestartSheet from '../components/RestartSheet.svelte';
  import MessageSheet from '../components/MessageSheet.svelte';
  import CodName from '../components/CodName.svelte';
  import ServerDetails from '../components/ServerDetails.svelte';
  import VoteSheet from '../components/VoteSheet.svelte';
  import XpSheet from '../components/XpSheet.svelte';
  import { get } from '../lib/api.js';
  import { app, currentServer, mapName } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { meOn } from '../lib/me.svelte.js';
  import { go, href } from '../lib/router.svelte.js';
  import { teamName, ago } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';
  import { say } from '../lib/toast.svelte.js';

  const srv = $derived(currentServer());
  const id = $derived(app.serverId);
  const perms = $derived(permsOf(app.me, id));
  const canPrivate = $derived(perms.has('private'));

  let status = $state(null);
  let feed = $state(null);
  let week = $state(null);
  let history = $state(null);
  let rconOn = $state(false);
  let chat = $state([]);
  let now = $state(Date.now() / 1000);
  let view = $state('players');
  let sheet = $state('');
  let selected = $state(null);
  let msgTarget = $state(null);
  let mapImg = $state(true);
  let uptime = $state(null);
  /** @type {any[]} */
  let notes = $state([]);
  let xp = $state(''); // xp_scale of pm_sniper ('' = not readable: no tool)
  let voteOn = $state(false);

  async function loadChat(sid) {
    try {
      const r = await get(`/api/servers/${sid}/chat?limit=40`);
      if (sid === app.serverId) chat = r.messages ?? [];
    } catch {}
  }

  // Everything of the selected server; the live part every 10 s while the page is visible.
  $effect(() => {
    const sid = id;
    if (!sid) return;
    let alive = true;
    status = feed = week = history = null;
    chat = [];
    rconOn = false;
    const base = `/api/servers/${sid}`;
    const live = async () => {
      const [s, f] = await Promise.all([get(base + '/status').catch(() => null), get(base + '/feed').catch(() => null)]);
      if (!alive) return;
      status = s;
      feed = f;
      now = Date.now() / 1000;
      if (untrack(() => view === 'chat' && canPrivate)) loadChat(sid);
    };
    live();
    get(base + '/week').then(w => { if (alive) week = w; }).catch(() => {});
    get(base + '/history?range=24h').then(h => { if (alive) history = h; }).catch(() => {});
    get(base + '/rcon').then(r => { if (alive) rconOn = !!r.enabled; }).catch(() => {});
    get(base + '/uptime').then(u => { if (alive) uptime = u; }).catch(() => {});
    const every = setInterval(() => { if (!document.hidden) live(); }, 10000);
    return () => { alive = false; clearInterval(every); };
  });

  $effect(() => { if (view === 'chat' && canPrivate && id) loadChat(id); });

  // staff tags of the notes, shown next to the connected players
  $effect(() => {
    const sid = id;
    notes = [];
    if (!sid || !canPrivate) return;
    get(`/api/servers/${sid}/notes`).then(r => { if (sid === app.serverId) notes = r.notes ?? []; }).catch(() => {});
  });
  const tags = $derived.by(() => {
    const m = new Map();
    for (const n of notes) {
      if (!n.tag) continue;
      m.set(n.key, n.tag);
      if (n.key.startsWith('n:')) m.set(n.key.slice(2), n.tag);
      for (const name of String(n.names ?? '').split('|').filter(Boolean)) m.set(name, n.tag);
    }
    return m;
  });
  const tagOf = p => tags.get(p.key) ?? tags.get(p.name_clean) ?? '';

  // tools of the pm_sniper mod (rcon permission): shown when the server answers their dvar
  function loadXp(sid) {
    get(`/api/servers/${sid}/dvar?name=xp_scale`).then(r => { if (sid === app.serverId) xp = r.value ?? ''; }).catch(() => {});
  }
  $effect(() => {
    const sid = id;
    xp = '';
    voteOn = false;
    if (!sid || !rconOn || !perms.has('rcon')) return;
    loadXp(sid);
    get(`/api/servers/${sid}/dvar?name=scr_mapvote_maps`).then(r => { if (sid === app.serverId) voteOn = r.value != null; }).catch(() => {});
  });
  $effect(() => { if (view === 'chat' && !canPrivate) view = 'players'; });
  $effect(() => { srv?.map; mapImg = true; });

  const online = $derived(!!srv?.online);
  const map = $derived(srv?.map ?? '');
  const match = $derived(feed?.match ?? null);
  const hasScore = $derived(match && match.score_allies != null);

  // Players of the status, completed with what the logs say (team, kills, deaths, key).
  const players = $derived.by(() => {
    const seen = feed?.players ?? {};
    return (status?.players ?? []).map(p => ({ ...p, ...(seen[p.name_raw] ?? {}) }));
  });
  const groups = $derived.by(() => {
    if (!players.length) return [];
    const by = id => players.filter(p => (p.team || '') === id).sort((a, b) => (b.score ?? 0) - (a.score ?? 0));
    const teams = [
      { id: 'allies', name: teamName('allies'), color: 'var(--allies)' },
      { id: 'axis', name: teamName('axis'), color: 'var(--axis)' },
      { id: '', name: t('En jeu'), color: 'var(--muted)' }
    ].map(g => ({ ...g, players: by(g.id) })).filter(g => g.players.length);
    return teams; // nobody's team known (no logs): one plain "En jeu" list
  });

  // "C'est moi": this phone's player, when they are in game
  const me = $derived(meOn(id));
  const meLive = $derived(me ? players.find(p => p.key === me.key) ?? null : null);

  const staffActions = $derived(rconOn && (perms.has('maps') || perms.has('say') || perms.has('rcon')));
  const canTapAct = $derived(rconOn && (perms.has('kick') || perms.has('ban') || perms.has('say')));
  const address = $derived.by(() => {
    if (!srv) return '';
    const local = /^(127\.|localhost$|::1$)/.test(srv.host);
    return `${local ? location.hostname : srv.host}:${srv.port}`;
  });

  async function join() {
    const cmd = `/connect ${address}`;
    try {
      await navigator.clipboard.writeText(cmd);
      say(t('Copié : {cmd}', { cmd }));
    } catch {
      say(cmd);
    }
  }

  async function share() {
    const text = `${srv.name.replace(/\^\d/g, '')} · /connect ${address}`;
    try {
      if (navigator.share) await navigator.share({ title: srv.name.replace(/\^\d/g, ''), text, url: location.href });
      else { await navigator.clipboard.writeText(text); say(t('Copié : {cmd}', { cmd: text })); }
    } catch {}
  }

  function tap(p) {
    if (canTapAct) { selected = p; sheet = 'player'; }
    else if (p.key) go('joueur', p.key);
  }
  const close = () => { sheet = ''; };
  const segs = $derived([
    ['players', `${t('Joueurs')} (${players.length})`],
    ['feed', t('Fil du match')],
    ...(canPrivate ? [['chat', t('Chat')]] : [])
  ]);
  const lastOk = $derived(status?.last_ok?.[0]?.ts ?? null);
</script>

{#if !srv}
  <p class="muted">{app.loaded ? t('Aucun serveur surveillé pour le moment.') : t('Chargement…')}</p>
{:else}
<div class="layout">
<div class="stack">
  {#if online}
    <section class="card hero" aria-label={srv.name}>
      <div class="top">
        {#if map && mapImg}<img src={`/maps/${map}.jpg`} alt="" onerror={() => (mapImg = false)} />{/if}
        <span class="code" aria-hidden="true">{mapName(map).slice(0, 3).toUpperCase()}</span>
        <div class="chips">
          <span class="chip live"><span class="dot"></span>{t('En ligne')}</span>
          {#if srv.gametype}<span class="chip">{srv.gametype.toUpperCase()}</span>{/if}
          <span class="chip right">{srv.players ?? 0}/{srv.max_players ?? '?'} {t('joueurs')}</span>
        </div>
        <div class="map">
          <div class="label">{t('Map')}</div>
          <div class="mapname">{mapName(map)}</div>
        </div>
      </div>
      {#if hasScore}
        <div class="score">
          <div><div class="side">{teamName('allies').toUpperCase()}</div><div class="pts allies">{match.score_allies}</div></div>
          <div class="mid"><div class="side">{t('Round')}</div><div class="rnd">{(match.rounds ?? 0) + 1}</div></div>
          <div class="r"><div class="side">{teamName('axis').toUpperCase()}</div><div class="pts axis">{match.score_axis}</div></div>
        </div>
      {/if}
      <div class="actions">
        <button class="btn primary" onclick={join}><Icon name="arrowRight" size={20} stroke={2.2} />{t('Rejoindre la partie')}</button>
        <button class="btn square" onclick={share} aria-label={t('Partager le serveur')}><Icon name="share" size={20} /></button>
      </div>
    </section>
  {:else}
    <section class="card down">
      <span class="chip off"><span class="dot"></span>{t('Hors ligne')}</span>
      <h1>{t('Le serveur ne répond plus')}</h1>
      <p class="muted">{lastOk ? t('Dernière réponse il y a {d}.', { d: ago(lastOk, now) }) : t('Aucune réponse enregistrée pour le moment.')}
        {#if canPrivate}{t('Les admins sont prévenus par mail.')}{/if}</p>
      {#if canPrivate}<a class="btn" href="#/etat">{t('Voir les coupures et la cause')}</a>{/if}
    </section>
  {/if}

  {#if online && meLive}
    <a class="card melive" href={href('joueur', me.key)}>
      <span class="av">{(meLive.name_clean || '?').charAt(0).toUpperCase()}</span>
      <span class="ml">
        <span class="small">{t('Tu es en jeu')}{#if meLive.team}{' · '}{teamName(meLive.team)}{/if}</span>
        <span class="line"><b><CodName raw={meLive.name_raw} /></b><span>{t('{k} kills · {d} morts · score {s}', { k: meLive.kills ?? 0, d: meLive.deaths ?? 0, s: meLive.score ?? 0 })}</span></span>
      </span>
      <Icon name="chevronRight" />
    </a>
  {/if}

  {#if online && staffActions}
    <div class="quick">
      {#if perms.has('maps')}
        <button onclick={() => (sheet = 'maps')}><Icon name="map" />{t('Changer de map')}</button>
        <button onclick={() => (sheet = 'restart')}><Icon name="refresh" />{t('Relancer')}</button>
      {/if}
      {#if perms.has('say')}
        <button onclick={() => { msgTarget = null; sheet = 'message'; }}><Icon name="message" />{t('Message à tous')}</button>
      {/if}
      {#if voteOn}
        <button onclick={() => (sheet = 'vote')}><Icon name="list" />{t('Maps du vote')}</button>
      {/if}
      {#if xp}
        <button onclick={() => (sheet = 'xp')}><Icon name="star" />{t('Boost XP')} <span class="xp" class:boost={+xp > 1}>x{xp}</span></button>
      {/if}
    </div>
  {/if}

  {#if online}
    <div class="seg" role="tablist" aria-label={t('Vue du direct')}>
      {#each segs as [key, label] (key)}
        <button role="tab" aria-selected={view === key} class:on={view === key} onclick={() => (view = key)}>{label}</button>
      {/each}
    </div>

    {#if view === 'players'}
      {#if players.length}
        <LivePlayers {groups} ontap={tap} meKey={me?.key ?? ''} {tagOf} {now}
          hint={canTapAct ? t('Appuie sur un joueur pour lui écrire, l\'expulser ou le bannir.') : ''} />
      {:else}
        <section class="card empty">
          <h2>{t('Personne en jeu pour le moment')}</h2>
          <p class="muted">{t('Le serveur est prêt : rejoins-le, les autres suivront !')}</p>
        </section>
      {/if}
    {:else if view === 'feed'}
      <MatchFeed events={feed?.events ?? []} {now} />
    {:else if view === 'chat'}
      <LiveChat messages={chat} {now} canSay={rconOn && perms.has('say')} serverId={id} onsent={() => setTimeout(() => loadChat(id), 1500)} />
    {/if}
  {/if}

</div>
<aside class="stack">
  <ServerDetails {status} {uptime} {now} />
  <TodayCard {history} playersNow={online ? srv.players ?? 0 : 0} />
  <WeekCard {week} />
</aside>
</div>

<PlayerSheet open={sheet === 'player'} onclose={close} player={selected} serverId={id} {perms}
  onmessage={p => { msgTarget = p; sheet = 'message'; }} />
<MapSheet open={sheet === 'maps'} onclose={close} serverId={id} current={map} gametype={srv.gametype ?? ''}
  mod={status?.vars?.fs_game ?? ''} players={players.length} canMode={perms.has('rcon')} />
<RestartSheet open={sheet === 'restart'} onclose={close} serverId={id} mapTitle={mapName(map)} canRcon={perms.has('rcon')} />
<VoteSheet open={sheet === 'vote'} onclose={close} serverId={id} />
<XpSheet open={sheet === 'xp'} onclose={close} serverId={id} current={xp} ondone={() => loadXp(id)} />
<MessageSheet open={sheet === 'message'} onclose={close} serverId={id} target={msgTarget} />
{/if}

<style>
  .stack { display: flex; flex-direction: column; gap: 14px; }
  .melive { display: flex; align-items: center; gap: 12px; padding: 12px 14px; text-decoration: none; color: var(--text); border-color: var(--accent); background: var(--accent-soft); }
  .melive .av { width: 40px; height: 40px; flex: none; border-radius: 12px; background: var(--accent); color: var(--on-accent); display: flex; align-items: center; justify-content: center; font-weight: 800; }
  .ml { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .ml .small { font-size: 12px; font-weight: 800; color: var(--text-2); }
  .ml .line { display: flex; gap: 6px; min-width: 0; font-size: 14px; white-space: nowrap; overflow: hidden; }
  .ml .line b { display: flex; min-width: 0; }
  .ml .line span { color: var(--text-2); overflow: hidden; text-overflow: ellipsis; }
  .hero { overflow: hidden; }
  .top { position: relative; height: 136px; background: var(--surface-2); padding: 14px 16px; display: flex; flex-direction: column; justify-content: space-between; overflow: hidden; }
  .top img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; opacity: .45; }
  .top::after { content: ''; position: absolute; inset: 0; background: linear-gradient(180deg, rgba(11, 14, 19, .15), rgba(20, 25, 34, .92)); }
  .code { position: absolute; right: -4px; bottom: -30px; font-family: var(--font-display); font-weight: 700; font-size: 112px; letter-spacing: -6px; color: rgba(255, 255, 255, .05); z-index: 1; }
  .chips, .map { position: relative; z-index: 2; }
  .chips { display: flex; gap: 6px; align-items: center; }
  .chip { display: flex; align-items: center; gap: 6px; height: 26px; padding: 0 10px; border-radius: 13px; background: rgba(38, 48, 66, .9); color: var(--text-2); font-size: 12px; font-weight: 800; }
  .chip.right { margin-left: auto; color: var(--text); }
  .chip.live { background: rgba(23, 60, 44, .9); color: var(--ok); }
  .chip.off { background: rgba(255, 99, 105, .14); color: var(--danger-text); align-self: flex-start; }
  .chip .dot { width: 7px; height: 7px; border-radius: 4px; background: currentColor; }
  .label { font-size: 11px; color: var(--text-2); text-transform: uppercase; letter-spacing: .1em; font-weight: 700; }
  .mapname { font-family: var(--font-display); font-size: 32px; font-weight: 700; line-height: 1.05; }
  .score { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); align-items: center; padding: 14px 16px 2px; }
  .side { font-size: 11px; color: var(--muted); font-weight: 800; letter-spacing: .08em; }
  .pts { font-family: var(--font-display); font-size: 36px; font-weight: 700; line-height: 1.1; }
  .pts.allies { color: var(--allies); }
  .pts.axis { color: var(--axis); }
  .mid { text-align: center; }
  .rnd { font-family: var(--font-display); font-size: 20px; font-weight: 700; }
  .r { text-align: right; }
  .actions { padding: 14px 16px 16px; display: flex; gap: 8px; }
  .actions .primary { flex: 1; }
  .square { width: 48px; padding: 0; }
  .down { padding: 18px 16px; display: flex; flex-direction: column; gap: 10px; border-color: #3A2328; align-items: flex-start; }
  .down h1 { font-family: var(--font-display); font-size: 24px; }
  .down p { margin: 0; font-size: 14px; }
  .quick { display: grid; grid-template-columns: repeat(auto-fit, minmax(96px, 1fr)); gap: 8px; }
  .quick button { height: 72px; border-radius: 16px; border: 1px solid var(--line); background: var(--surface); display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 6px; font-size: 12px; font-weight: 700; }
  .quick .xp { font-size: 11px; color: var(--muted); }
  .quick .xp.boost { color: var(--accent); }
  .seg { display: flex; gap: 4px; padding: 4px; background: var(--surface); border: 1px solid var(--line); border-radius: 14px; }
  .seg button { flex: 1; height: 38px; border: 0; border-radius: 10px; background: transparent; color: var(--text-2); font-weight: 700; font-size: 13px; }
  .seg button.on { background: var(--accent); color: var(--on-accent); }
  .empty { padding: 18px 16px; }
  .empty h2 { font-size: 17px; }
  .empty p { margin: 4px 0 0; font-size: 14px; }
  .layout { display: flex; flex-direction: column; gap: 14px; }
  @media (min-width: 960px) {
    .layout { display: grid; grid-template-columns: minmax(0, 3fr) minmax(320px, 2fr); align-items: start; gap: 18px; }
  }
</style>
