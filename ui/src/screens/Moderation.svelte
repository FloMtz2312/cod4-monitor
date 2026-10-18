<script>
  // Modération: the inbox of what the staff has to handle (reports, suspicious stats, tagged players in game,
  // possible ban evasions, B3 bans), each with its actions; then the history (handled reports, B3 penalties).
  import CodName from '../components/CodName.svelte';
  import Seg from '../components/Seg.svelte';
  import Icon from '../components/Icon.svelte';
  import PlayerSheet from '../components/PlayerSheet.svelte';
  import MessageSheet from '../components/MessageSheet.svelte';
  import { get, post } from '../lib/api.js';
  import { app } from '../lib/state.svelte.js';
  import { permsOf, canAct, isOwner } from '../lib/perms.js';
  import { inbox, loadInbox, dismiss } from '../lib/inbox.svelte.js';
  import { chatUnread, chatMentions } from '../lib/adminchat.svelte.js';
  import { href } from '../lib/router.svelte.js';
  import { agoText, clean } from '../lib/format.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';
  import { viewerURL, viewerInstalled } from '../lib/demos.js';

  const id = $derived(app.serverId);
  const perms = $derived(permsOf(app.me, id));
  const owner = $derived(isOwner(app.me, id));
  const canClose = $derived(perms.has('reports'));

  let view = $state('todo');
  let filter = $state('all');
  let live = $state([]);
  let rconOn = $state(false);
  let reports = $state(null);
  let penalties = $state(null);
  let busy = $state('');
  let confirmLift = $state('');
  let viewer = $state(false);
  viewerInstalled().then(v => { viewer = v; });
  let sheet = $state('');
  let target = $state(null);

  $effect(() => {
    const sid = id;
    if (!sid) return;
    let alive = true;
    live = [];
    rconOn = false;
    loadInbox(sid);
    const base = `/api/servers/${sid}`;
    const status = () => get(`${base}/status`).then(r => { if (alive) live = r.players ?? []; }).catch(() => {});
    status();
    get(`${base}/rcon`).then(r => { if (alive) rconOn = !!r.enabled; }).catch(() => {});
    const every = setInterval(() => { if (!document.hidden) { loadInbox(sid); status(); } }, 30000);
    return () => { alive = false; clearInterval(every); };
  });

  $effect(() => {
    const sid = id;
    if (!sid || view !== 'history') return;
    let alive = true;
    reports = penalties = null;
    get(`/api/servers/${sid}/reports`).then(r => { if (alive) reports = (r.reports ?? []).filter(x => x.handled_at).slice(0, 30); }).catch(() => { if (alive) reports = []; });
    if (inbox.b3 && inbox.private) {
      get(`/api/servers/${sid}/b3/penalties?limit=40`).then(r => { if (alive) penalties = r.penalties ?? []; }).catch(() => { if (alive) penalties = []; });
    }
    return () => { alive = false; };
  });

  const items = $derived(inbox.serverId === id ? inbox.items : null);
  const TYPES = [
    ['report', 'Signalements'], ['suspect', 'Stats suspectes'], ['watch', 'À surveiller'], ['evasion', 'Contournements'], ['b3ban', 'Bans B3']
  ];
  const LABEL = { report: 'Signalement', suspect: 'Stats suspectes', watch: 'À surveiller', evasion: 'Contournement de ban', b3ban: 'Ban B3' };
  const counts = $derived(Object.fromEntries(TYPES.map(([k]) => [k, (items ?? []).filter(it => it.type === k).length])));
  const shown = $derived((items ?? []).filter(it => filter === 'all' || it.type === filter));
  $effect(() => { if (filter !== 'all' && !counts[filter]) filter = 'all'; });

  // the player online now (status of the server), for the actions in game
  const onlineOf = it => {
    const names = new Set([clean(it.name_raw), clean(it.target)].filter(Boolean).map(n => n.toLowerCase()));
    return live.find(p => names.has((p.name_clean ?? '').toLowerCase())) ?? null;
  };
  const canTapAct = $derived(rconOn && canAct(perms));

  const REASON = { ip: 'même IP', subnet: 'même sous-réseau', steam: 'même compte Steam', profile: 'même profil de jeu', settings: 'mêmes réglages réseau',
    name: 'même pseudo', name_near: 'pseudo proche', style: 'même style de jeu', veteran: 'déjà expérimenté', new: 'GUID nouveau' };
  const BAN = { Ban: 'banni définitivement', TempBan: 'banni temporairement' };
  const PEN = { Warning: 'Avertissement', Kick: 'Expulsion', TempBan: 'Ban temporaire', Ban: 'Ban', Notice: 'Note B3' };

  async function close(it) {
    busy = it.id;
    try {
      await dismiss(id, it.id);
      say(it.type === 'report' ? t('Signalement traité') : t('Classé'));
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    } finally {
      busy = '';
    }
  }

  async function lift(pid, itemId) {
    busy = itemId || String(pid);
    try {
      await post(`/api/servers/${id}/b3/penalty/${pid}/lift`, {});
      say(t('Ban levé'));
      if (itemId) await dismiss(id, itemId);
      if (penalties) penalties = penalties.map(p => (p.id === pid ? { ...p, active: false } : p));
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    } finally {
      busy = '';
      confirmLift = '';
    }
  }

  // the moderation tools this account may open
  const tools = $derived([
    !app.me?.noauth && ['chat-admins', 'users', 'Chat des admins'],
    perms.has('private') && ['chat', 'message', 'Chat'],
    (perms.has('private') || perms.has('rcon')) && ['console', 'terminal', 'Console'],
    perms.has('ban') && ['bans', 'ban', 'Bans'],
    perms.has('private') && ['contournements', 'alert', 'Contournements'],
    perms.has('private') && ['comptes-lies', 'link', 'Comptes liés'],
    perms.has('private') && ['demos', 'film', 'Démos'],
    perms.has('private') && inbox.b3 && inbox.serverId === id && ['b3', 'bot', 'B3']
  ].filter(x => !!x));

  function act(p) {
    target = { ...p };
    sheet = 'player';
  }
</script>

<div class="head">
  <h1 class="title">{t('Modération')}</h1>
  <p class="muted">
    {#if items === null}{t('Chargement…')}{:else if items.length}{t(items.length > 1 ? '{n} éléments à traiter' : '{n} élément à traiter', { n: items.length })}{:else}{t('Rien à traiter')}{/if}
  </p>
</div>

{#if tools.length}
  <nav class="tools" aria-label={t('Outils')}>
    {#each tools as [name, icon, label] (name)}<a href={href(name)}><Icon name={icon} size={18} />{t(label)}{#if name === 'chat-admins' && chatUnread(id)}<span class="dot" class:mention={chatMentions(id) > 0}>{chatMentions(id) ? '@' + chatMentions(id) : chatUnread(id)}</span>{/if}</a>{/each}
  </nav>
{/if}

<Seg label={t('Vue')} value={view} onchange={v => (view = v)} items={[['todo', t('À traiter')], ['history', t('Historique')]]} />

{#if view === 'todo'}
  {#if items?.length}
    <div class="chips" role="group" aria-label={t('Filtrer')}>
      <button aria-pressed={filter === 'all'} onclick={() => (filter = 'all')}>{t('Tout')} {items.length}</button>
      {#each TYPES as [k, label] (k)}
        {#if counts[k]}<button aria-pressed={filter === k} onclick={() => (filter = k)}>{t(label)} {counts[k]}</button>{/if}
      {/each}
    </div>
  {/if}

  {#if items && !items.length}
    <section class="card done">
      <span class="ok"><Icon name="check" size={26} /></span>
      <b>{t('Tout est traité')}</b>
      <span class="muted">{t('Les nouveaux signalements et alertes apparaîtront ici.')}</span>
    </section>
  {/if}

  <div class="list">
    {#each shown as it (it.id)}
      {@const on = onlineOf(it)}
      <article class="card item">
        <div class="top">
          <span class="pill {it.type}">{t(LABEL[it.type])}</span>
          <span class="muted meta">{it.type === 'watch' ? t('en jeu maintenant') : agoText(it.ts)}</span>
        </div>

        {#if it.type === 'report'}
          <h2><CodName raw={it.name_raw || it.target} /> <span class="muted">{t('signalé par')}</span> <CodName raw={it.reporter_name} /></h2>
          <p>{it.reason ? `« ${it.reason} »` : t('Sans raison.')}{#if on}{' '}{t('Il est en ligne.')}{/if}</p>
        {:else if it.type === 'suspect'}
          <h2><CodName raw={it.name_raw} /> : {#if it.why.includes('hs')}{t('{p} % de headshots', { p: Math.round(100 * it.hs_rate) })}{:else}{t('K/D {kd}', { kd: (it.kills / Math.max(1, it.deaths)).toFixed(1) })}{/if}</h2>
          <p>{t('{hs} headshots sur {k} kills, {d} morts en 7 jours. Les autres joueurs : {p} % de headshots.', { hs: it.headshots, k: it.kills, d: it.deaths, p: Math.round(100 * it.server_hs_rate) })}</p>
        {:else if it.type === 'watch'}
          <h2><CodName raw={it.name_raw} /> {t('est en jeu')}</h2>
          <p>{it.tag === 'suspect' ? t('Noté « suspect » par l\'équipe') : t('Noté « à surveiller » par l\'équipe')}{#if it.note}{' : '}{it.note}{/if}</p>
        {:else if it.type === 'evasion'}
          <h2><CodName raw={it.name_raw} /> <span class="muted">{t('ressemble à')}</span> <CodName raw={it.ban_name} /></h2>
          <p>
            {#if it.source === 'b3'}{t('B3 : même IP que ce joueur banni{r}.', { r: it.reason ? ` (${it.reason})` : '' })}
            {:else}{t('Score {s} : {why}.', { s: it.score, why: (it.reasons ?? []).map(r => t(REASON[r.code] ?? r.code)).join(', ') })}{/if}
          </p>
        {:else if it.type === 'b3ban'}
          <h2><CodName raw={it.name_raw} /> {t(BAN[it.ban_type] ?? 'banni')} {#if it.admin}<span class="muted">{t('par')}</span> {it.admin}{/if}</h2>
          <p>{t('Raison : {r}', { r: it.reason || it.keyword || '—' })}{#if it.ban_type === 'TempBan' && it.expire > 0}{' · '}{t('jusqu\'au {d}', { d: new Date(it.expire * 1000).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' }) })}{/if}{#if !it.active}{' · '}{t('levé')}{/if}</p>
        {/if}

        <div class="acts">
          {#if it.key}<a class="btn" href={href('joueur', it.key)}>{t('Voir')}</a>{/if}
          {#if it.type === 'evasion' && it.ban_key}<a class="btn" href={href('joueur', it.ban_key)}>{t('Voir le banni')}</a>{/if}
          {#if it.demos?.length === 1 && viewer}<a class="btn" href={viewerURL(id, it.demos[0])} target="_blank" rel="noopener"><Icon name="play" size={16} />{t('Voir la démo')}</a>
          {:else if it.demos?.length}<a class="btn" href={href('demos', it.key)}><Icon name="film" size={16} />{t('Démos ({n})', { n: it.demos.length })}</a>{/if}
          {#if on && canTapAct && it.type !== 'b3ban'}<button class="btn primary" onclick={() => act({ ...on, key: it.key })}>{t('Agir en jeu')}</button>{/if}
          {#if it.type === 'b3ban' && it.active && owner}
            {#if confirmLift === it.id}
              <button class="btn danger" disabled={busy === it.id} onclick={() => lift(it.pid, it.id)}>{t('Confirmer')}</button>
            {:else}
              <button class="btn" onclick={() => (confirmLift = it.id)}>{t('Lever le ban')}</button>
            {/if}
          {/if}
          {#if canClose}
            <button class="btn" disabled={busy === it.id} onclick={() => close(it)}>{it.type === 'b3ban' || it.type === 'watch' ? t('OK') : t('Classer')}</button>
          {/if}
        </div>
      </article>
    {/each}
  </div>
{:else}
  <section class="card box" aria-label={t('Signalements traités')}>
    <h2 class="label">{t('Signalements traités')}</h2>
    <p class="muted cmds">{t('Commandes du chat du jeu : !report <joueur> <raison> pour signaler, !stats et !me pour ses stats, !top pour le classement, !help pour l\'aide (réponses en message privé, il faut le mot de passe rcon).')}</p>
    {#if reports === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else}
      {#each reports as r (r.id)}
        <div class="hrow">
          <span class="ht"><b>{r.target}</b> <span class="muted">· {r.reason || '—'}</span></span>
          <span class="muted hs">{t('par {who}, {when}', { who: r.handled_by || '—', when: agoText(r.handled_at) })}</span>
        </div>
      {:else}
        <p class="muted">{t('Aucun signalement traité.')}</p>
      {/each}
    {/if}
  </section>

  {#if inbox.b3 && inbox.private}
    <section class="card box" aria-label={t('Sanctions B3')}>
      <h2 class="label">{t('Sanctions B3 récentes')}</h2>
      {#if penalties === null}
        <p class="muted">{t('Chargement…')}</p>
      {:else}
        {#each penalties as p (p.id)}
          <div class="hrow">
            <span class="ht">
              <span class="kind" class:active={p.active}>{t(PEN[p.type] ?? p.type)}</span>
              <a href={href('joueur', p.guid)}><CodName raw={p.name} /></a>
              <span class="muted">· {p.reason || p.keyword || '—'}</span>
            </span>
            <span class="muted hs">{p.admin || 'B3'} · {agoText(p.time_add)}
              {#if owner && p.active && (p.type === 'Ban' || p.type === 'TempBan')}
                {#if confirmLift === String(p.id)}
                  <button class="link danger" disabled={busy === String(p.id)} onclick={() => lift(p.id, '')}>{t('Confirmer')}</button>
                {:else}
                  <button class="link" onclick={() => (confirmLift = String(p.id))}>{t('Lever')}</button>
                {/if}
              {/if}
            </span>
          </div>
        {:else}
          <p class="muted">{t('Aucune sanction.')}</p>
        {/each}
      {/if}
    </section>
  {/if}
{/if}

{#if target}
  <PlayerSheet open={sheet === 'player'} onclose={() => (sheet = '')} player={target} serverId={id} {perms} onmessage={() => (sheet = 'message')} />
  <MessageSheet open={sheet === 'message'} onclose={() => (sheet = '')} serverId={id} {target} />
{/if}

<style>
  .head { display: flex; align-items: baseline; justify-content: space-between; gap: 10px; margin-bottom: 14px; flex-wrap: wrap; }
  .head p { margin: 0; font-size: 14px; }
  .tools { display: flex; gap: 6px; overflow-x: auto; margin: 0 0 12px; padding-bottom: 2px; scrollbar-width: none; }
  .tools a { flex: none; display: inline-flex; align-items: center; gap: 7px; height: 40px; padding: 0 13px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); color: var(--text); text-decoration: none; font-weight: 700; font-size: 13px; }
  .tools a :global(svg) { color: var(--accent); }
  .dot { min-width: 20px; height: 20px; padding: 0 6px; border-radius: 10px; background: var(--surface-2); color: var(--text); font-size: 11px; font-weight: 800; display: inline-flex; align-items: center; justify-content: center; }
  .dot.mention { background: var(--danger); color: #fff; }
  .cmds { font-size: 12px; }
  .chips { display: flex; gap: 6px; overflow-x: auto; margin: 12px 0 0; padding-bottom: 2px; scrollbar-width: none; }
  .chips button { flex: none; height: 36px; padding: 0 12px; border-radius: 18px; border: 1px solid var(--line); background: transparent; color: var(--text-2); font-weight: 700; font-size: 13px; }
  .chips button[aria-pressed='true'] { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .done { margin-top: 14px; padding: 28px 16px; display: flex; flex-direction: column; align-items: center; gap: 6px; text-align: center; }
  .ok { width: 52px; height: 52px; border-radius: 16px; background: rgba(61, 214, 140, .14); color: var(--ok); display: flex; align-items: center; justify-content: center; margin-bottom: 4px; }
  .list { display: grid; gap: 10px; margin-top: 12px; }
  .item { padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .top { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .pill { font-size: 12px; font-weight: 800; padding: 4px 9px; border-radius: 8px; }
  .pill.report { background: rgba(245, 196, 81, .15); color: var(--warn); }
  .pill.suspect { background: rgba(255, 99, 105, .15); color: var(--danger-text); }
  .pill.watch { background: rgba(245, 165, 36, .15); color: var(--accent); }
  .pill.evasion { background: rgba(185, 140, 255, .16); color: #C7A6FF; }
  .pill.b3ban { background: rgba(94, 160, 255, .15); color: #8DBBFF; }
  .meta { font-size: 12px; }
  h2 { font-size: 16px; font-weight: 800; display: flex; flex-wrap: wrap; gap: 5px; align-items: baseline; }
  h2 .muted { font-weight: 600; }
  .item p { margin: 0; font-size: 14px; color: var(--text-2); overflow-wrap: anywhere; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; margin-top: 4px; }
  .acts .btn { min-height: 40px; padding: 0 14px; font-size: 14px; }
  .box { margin-top: 12px; padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; }
  .box p { margin: 0; }
  .hrow { display: flex; flex-direction: column; gap: 2px; padding-top: 8px; border-top: 1px solid var(--line-soft); font-size: 14px; }
  .label + .hrow { border-top: 0; padding-top: 0; }
  .ht { display: flex; flex-wrap: wrap; gap: 5px; align-items: center; min-width: 0; }
  .ht a { text-decoration: none; color: var(--text); font-weight: 700; display: inline-flex; }
  .hs { font-size: 12px; display: flex; gap: 8px; align-items: center; }
  .kind { font-size: 11px; font-weight: 800; padding: 2px 7px; border-radius: 7px; background: var(--surface-2); color: var(--text-2); }
  .kind.active { background: rgba(255, 99, 105, .14); color: var(--danger-text); }
  .link { border: 0; background: none; color: var(--accent); font-weight: 800; font-size: 12px; padding: 0; }
  .link.danger { color: var(--danger-text); }
  @media (min-width: 960px) { .list { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; } }
</style>
