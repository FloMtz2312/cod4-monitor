<script>
  // Modération tab of a profile (staff with the private data): actions in game, welcome message (owner), B3,
  // identity (names, IPs, accounts on the same IP, bans), sessions, the staff note and the chat.
  import CodName from '../CodName.svelte';
  import PlayerSheet from '../PlayerSheet.svelte';
  import MessageSheet from '../MessageSheet.svelte';
  import PlayerB3 from './PlayerB3.svelte';
  import { api, get } from '../../lib/api.js';
  import { app } from '../../lib/state.svelte.js';
  import { canAct, isOwner } from '../../lib/perms.js';
  import { href } from '../../lib/router.svelte.js';
  import { teamName } from '../../lib/format.js';
  import { dt, dur } from '../../lib/dates.js';
  import { say } from '../../lib/toast.svelte.js';
  import { t } from '../../lib/i18n.svelte.js';
  import { length, live, viewerURL, viewerInstalled } from '../../lib/demos.js';

  /** @type {{ p: any, key: string, nameRaw: string, serverId: number, perms: Set<string>, livePlayer: any }} */
  let { p, key, nameRaw, serverId, perms, livePlayer } = $props();

  const hasGuid = $derived(!key.startsWith('n:'));
  const owner = $derived(isOwner(app.me, serverId));
  let rconOn = $state(false);
  let sheet = $state('');
  // the tags of the current interface: "suspect" and "surveiller" raise an alert when the player connects
  const TAGS = [['', 'Aucune'], ['suspect', '⚠ Suspect'], ['surveiller', '👁 À surveiller'], ['vip', '★ VIP'], ['ami', '♥ Ami']];
  let tag = $state('');
  let note = $state('');
  let noteAt = $state(0);
  let saving = $state(false);
  let welcome = $state('');
  let savingWelcome = $state(false);

  $effect(() => {
    const sid = serverId;
    rconOn = false;
    get(`/api/servers/${sid}/rcon`).then(r => { rconOn = !!r.enabled; }).catch(() => {});
  });

  // the demos recorded from this player's eyes (most recent first)
  let demos = $state([]);
  let viewer = $state(false);
  $effect(() => {
    const sid = serverId, k = key;
    demos = [];
    if (!priv || !hasGuid) return;
    let alive = true;
    get(`/api/servers/${sid}/demos?limit=6&player=${encodeURIComponent(k)}`).then(r => { if (alive) demos = r.demos ?? []; }).catch(() => {});
    viewerInstalled().then(v => { viewer = v; });
    return () => { alive = false; };
  });

  $effect(() => {
    const n = (Array.isArray(p.note) ? p.note[0] : p.note) ?? {};
    tag = n.tag ?? '';
    note = n.note ?? '';
    noteAt = n.updated_at ?? 0;
    welcome = p.welcome ?? '';
  });

  const priv = $derived(perms.has('private')); // without it: only the note (the notes permission)
  const ips = $derived(p.ips ?? []);
  const linked = $derived(p.linked ?? []);
  const sessions = $derived(p.sessions ?? []);
  const chat = $derived(p.chat ?? []);
  const now = Date.now() / 1000;

  async function saveNote(e) {
    e.preventDefault();
    saving = true;
    try {
      await api('PUT', `/api/servers/${serverId}/notes`, { key, tag: tag.trim(), note: note.trim() });
      noteAt = Math.floor(Date.now() / 1000);
      say(t('Note enregistrée'));
    } catch (err) {
      say(t('Échec : {err}', { err: err.message }));
    } finally {
      saving = false;
    }
  }

  async function saveWelcome(e) {
    e.preventDefault();
    savingWelcome = true;
    try {
      await api('PUT', `/api/servers/${serverId}/welcome`, { key, message: welcome.trim() });
      say(t('Message de bienvenue enregistré'));
    } catch (err) {
      say(t('Échec : {err}', { err: t(err.message) }));
    } finally {
      savingWelcome = false;
    }
  }
</script>

<div class="stack">
  {#if livePlayer && rconOn && canAct(perms)}
    <section class="card box live">
      <div><b>{t('En jeu en ce moment')}</b><span class="muted">{t('Message privé, expulsion, ban')}</span></div>
      <button class="btn primary" onclick={() => (sheet = 'player')}>{t('Actions')}</button>
    </section>
  {/if}

  {#if p.welcome !== undefined}
    <form class="card box" aria-label={t('Message de bienvenue')} onsubmit={saveWelcome}>
      <h2 class="label">{t('Message de bienvenue')}</h2>
      <p class="muted small">{t('Envoyé en message privé dans le jeu quand ce joueur se connecte. {name} = son pseudo. Vide = aucun message.')}</p>
      <div class="inline">
        <input bind:value={welcome} maxlength="150" placeholder={t('Bon retour {name} !')} aria-label={t('Message de bienvenue')} />
        <button class="btn" disabled={savingWelcome}>{t('Enregistrer')}</button>
      </div>
      <span class="muted small count">{welcome.length} / 150</span>
    </form>
  {/if}

  {#if priv && hasGuid}
    <div class="wide"><PlayerB3 {serverId} guid={key} {nameRaw} {owner} online={!!livePlayer} /></div>
  {/if}

  {#if priv}
  <section class="card box" aria-label={t('Identité')}>
    <h2 class="label">{t('Identité')}</h2>
    <dl>
      <dt>{t('Pseudos')}</dt>
      <dd class="names">{#each (p.names ?? []).slice(0, 20) as n (n.name_clean)}<span><CodName raw={n.name_raw} /></span>{:else}—{/each}</dd>
      <dt>{t('IP')}</dt>
      <dd>
        {#each ips as ip (ip.ip)}
          <div class="li"><span class="mono">{ip.ip}</span><span class="muted">{t('{n} sessions', { n: ip.sessions })}{' · '}{dt(ip.last_seen)}</span></div>
        {:else}—{/each}
      </dd>
      <dt>{t('Comptes liés (même IP)')}</dt>
      <dd>
        {#each linked as l, i (i)}
          <div class="li"><a href={href('joueur', l.key)}><CodName raw={l.name_raw || l.key} /></a><span class="muted"><span class="mono">{l.ip}</span>{' · '}{t('{n} sessions', { n: l.sessions })}</span></div>
        {:else}{t('aucun')}{/each}
      </dd>
      {#if p.bans?.length}
        <dt>{t('Bans du monitor')}</dt>
        <dd>
          {#each p.bans as b, i (i)}
            <div class="li"><span>{b.duration || t('définitif')} · {b.reason || '—'}</span><span class="muted">{dt(b.ts)}</span></div>
          {/each}
        </dd>
      {/if}
    </dl>
  </section>
  {/if}

  {#if priv && sessions.length}
    <section class="card box" aria-label={t('Sessions')}>
      <h2 class="label">{t('Sessions ({n} dernières)', { n: sessions.length })}</h2>
      <div class="scroll">
        {#each sessions as s, i (i)}
          <div class="ses">
            <div class="l1">
              <span>{dt(s.joined_at)}</span>
              <span class="arrow">→</span>
              {#if s.open}<span class="on">{t('en jeu')}</span>{:else if s.left_at}<span>{dt(s.left_at)}</span>{:else}<span class="muted">{t('inconnu')}</span>{/if}
              <span class="d">{s.left_at || s.open ? dur((s.left_at ?? now) - s.joined_at) : '—'}</span>
            </div>
            <div class="l2"><span class="nm"><CodName raw={s.name_raw} /></span><span class="mono muted">{s.ip}</span></div>
          </div>
        {/each}
      </div>
    </section>
  {/if}

  {#if demos.length}
    <section class="card box" aria-label={t('Démos')}>
      <div class="hd"><h2 class="label">{t('Démos')}</h2><a class="all" href={href('demos', key)}>{t('Toutes')}</a></div>
      {#each demos.slice(0, 5) as d (d.id)}
        <div class="li">
          <span>{dt(d.started_at)} · {length(d)}{#if live(d)}{' · '}<b class="rec">{t('en cours')}</b>{/if}{#if d.kept}{' · '}{t('gardée')}{/if}</span>
          {#if viewer}<a href={viewerURL(serverId, d)} target="_blank" rel="noopener">{t('Voir')}</a>{/if}
        </div>
      {/each}
    </section>
  {/if}

  <form class="card box" aria-label={t('Note de l\'équipe')} onsubmit={saveNote}>
    <h2 class="label">{t('Note de l\'équipe')}</h2>
    <p class="muted small">{t('Visible seulement par l\'équipe du serveur.')}{#if noteAt}{' '}{t('Modifiée le {d}.', { d: dt(noteAt) })}{/if}</p>
    <div class="tags" role="radiogroup" aria-label={t('Étiquette')}>
      {#each TAGS as [k, label] (k)}
        <button type="button" role="radio" aria-checked={tag === k} class:on={tag === k} disabled={!perms.has('notes')} onclick={() => (tag = k)}>{t(label)}</button>
      {/each}
      {#if tag && !TAGS.some(([k]) => k === tag)}<span class="kind">{tag}</span>{/if}
    </div>
    {#if tag === 'suspect' || tag === 'surveiller'}<p class="muted small">{t('Une alerte apparaît dans Modération (et par mail) quand il se connecte.')}</p>{/if}
    <textarea bind:value={note} maxlength="2000" rows="3" placeholder={t('Note…')} disabled={!perms.has('notes')}></textarea>
    {#if perms.has('notes')}<button class="btn" disabled={saving}>{t('Enregistrer la note')}</button>{/if}
  </form>

  {#if priv}
  <section class="card box chat" aria-label={t('Derniers messages')}>
    <div class="hd">
      <h2 class="label">{chat.length ? t('Chat ({n} derniers messages)', { n: chat.length }) : t('Chat')}</h2>
      <a class="all" href={href('chat', key)}>{t('Tout son chat')}</a>
    </div>
    {#if chat.length}
      <div class="scroll">
        {#each chat as m, i (i)}
          <div class="msg">
            <div class="l1"><span class="nm"><CodName raw={m.name_raw} /></span>{#if m.team}<span class="team">{t('(équipe)')}{#if teamName(m.team)}{' '}{teamName(m.team)}{/if}</span>{/if}<span class="muted when">{dt(m.ts)}</span></div>
            <div class="txt"><CodName raw={m.message} /></div>
          </div>
        {/each}
      </div>
    {:else}
      <p class="muted">{t('Aucun message')}</p>
    {/if}
  </section>
  {/if}
</div>

{#if livePlayer}
  <PlayerSheet open={sheet === 'player'} onclose={() => (sheet = '')} player={{ ...livePlayer, key: '' }} {serverId} {perms} onmessage={() => (sheet = 'message')} />
  <MessageSheet open={sheet === 'message'} onclose={() => (sheet = '')} {serverId} target={livePlayer} />
{/if}

<style>
  .stack { display: flex; flex-direction: column; gap: 12px; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .box p { margin: 0; }
  .live { flex-direction: row; align-items: center; justify-content: space-between; border-color: var(--ok); }
  .live div { display: flex; flex-direction: column; }
  .live .muted { font-size: 13px; }
  .hd { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .all { font-size: 13px; font-weight: 800; text-decoration: none; white-space: nowrap; }
  .inline { display: flex; gap: 8px; }
  .inline input { flex: 1; min-width: 0; height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .count { align-self: flex-end; }
  .kind { font-size: 12px; font-weight: 800; padding: 3px 8px; border-radius: 8px; background: var(--surface-2); color: var(--text-2); }
  dl { margin: 0; display: grid; grid-template-columns: minmax(0, 1fr); gap: 4px; }
  dt { font-size: 12px; color: var(--muted); font-weight: 700; margin-top: 6px; }
  dd { margin: 0; font-size: 14px; }
  .names { display: flex; flex-wrap: wrap; gap: 6px; }
  .names span { display: inline-flex; padding: 3px 8px; border-radius: 8px; background: var(--surface-2); font-weight: 700; max-width: 100%; }
  .li { display: flex; justify-content: space-between; align-items: baseline; gap: 10px; padding: 4px 0; border-top: 1px solid var(--line-soft); }
  .li:first-child { border-top: 0; }
  .li a { display: flex; min-width: 0; text-decoration: none; font-weight: 700; }
  .li .muted { font-size: 12px; text-align: right; }
  .mono { font-family: ui-monospace, monospace; overflow-wrap: anywhere; }
  .small { font-size: 12px; }
  .scroll { max-height: 340px; overflow-y: auto; margin: 0 -4px; padding: 0 4px; }
  .ses, .msg { padding: 8px 0; border-top: 1px solid var(--line-soft); font-size: 13px; }
  .ses:first-child, .msg:first-child { border-top: 0; }
  .ses .l1 { display: flex; flex-wrap: wrap; gap: 4px 6px; align-items: baseline; font-variant-numeric: tabular-nums; }
  .ses .arrow { color: var(--muted); }
  .ses .on { color: var(--ok); font-weight: 800; }
  .ses .d { margin-left: auto; font-weight: 800; }
  .l2 { display: flex; justify-content: space-between; gap: 8px; margin-top: 2px; }
  .nm { display: flex; min-width: 0; font-weight: 700; }
  .msg .l1 { display: flex; align-items: baseline; gap: 6px; min-width: 0; }
  .team { font-size: 11px; color: var(--muted); white-space: nowrap; }
  .when { margin-left: auto; font-size: 11px; white-space: nowrap; }
  .txt { margin-top: 2px; font-size: 14px; overflow-wrap: anywhere; }
  .txt :global(.name) { white-space: normal; }
  textarea { width: 100%; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 10px 12px; font-size: 16px; resize: vertical; }
  textarea:disabled { opacity: .7; }
  .rec { color: var(--danger-text); }
  .tags { display: flex; flex-wrap: wrap; gap: 6px; }
  .tags button { height: 36px; padding: 0 12px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); font-weight: 700; font-size: 13px; }
  .tags button.on { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .tags button:disabled { opacity: .7; }
  @media (min-width: 960px) {
    .stack { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; }
    .live, .wide { grid-column: 1 / -1; }
  }
</style>
