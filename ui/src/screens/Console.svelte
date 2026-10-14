<script>
  // Console of the selected server, two tabs: the game console (the last log lines, read every few seconds;
  // permission "private") and RCON (send a command, read its answer, the last commands with their output; "rcon").
  import BackTitle from '../components/BackTitle.svelte';
  import Seg from '../components/Seg.svelte';
  import { get, post } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { agoText, clean, colorParts } from '../lib/format.js';
  import { dt } from '../lib/moderation.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const srv = $derived(currentServer());
  const id = $derived(app.serverId);
  const perms = $derived(permsOf(app.me, id));
  const canRcon = $derived(perms.has('rcon'));
  const canLogs = $derived(perms.has('private'));
  const QUICK = ['status', 'serverinfo', 'map_rotate', 'fast_restart', 'g_gametype', 'sv_maxclients'];

  let tab = $state('');
  const view = $derived(tab === 'rcon' && canRcon ? 'rcon' : canLogs ? 'game' : canRcon ? 'rcon' : '');

  // --- game console
  let lines = $state(null);
  let logsOn = $state(true);
  let filter = $state('');
  let follow = $state(true);
  /** @type {HTMLDivElement | undefined} */
  let box = $state();

  $effect(() => {
    const sid = id;
    if (!sid || view !== 'game') return;
    let alive = true;
    lines = null;
    const load = () => get(`/api/servers/${sid}/logs`).then(r => {
      if (!alive) return;
      logsOn = !!r.enabled;
      lines = r.lines ?? [];
    }).catch(() => { if (alive && lines === null) lines = []; });
    load();
    const every = setInterval(() => { if (!document.hidden) load(); }, 4000);
    return () => { alive = false; clearInterval(every); };
  });

  const shown = $derived.by(() => {
    const f = filter.trim().toLowerCase();
    return (lines ?? []).filter(l => !f || clean(l).toLowerCase().includes(f));
  });

  // keep the newest line in view
  $effect(() => {
    shown;
    if (follow && box) box.scrollTop = box.scrollHeight;
  });

  // --- RCON
  let cmd = $state('');
  let output = $state(null);
  let busy = $state(false);
  let log = $state([]);
  let rconOn = $state(true);
  let open = $state(-1);

  const loadLog = sid => get(`/api/servers/${sid}/rcon/log`).then(r => { log = r.log ?? []; }).catch(() => {});
  $effect(() => {
    const sid = id;
    if (!sid || view !== 'rcon') return;
    output = null;
    open = -1;
    loadLog(sid);
    get(`/api/servers/${sid}/rcon`).then(r => { rconOn = !!r.enabled; }).catch(() => {});
  });

  async function send(e) {
    e?.preventDefault();
    const c = cmd.trim();
    if (!c || busy) return;
    busy = true;
    try {
      const r = await post(`/api/servers/${id}/rcon`, { command: c });
      output = { cmd: c, text: r.no_reply ? t('(pas de réponse du serveur)') : clean(r.output) };
      loadLog(id);
    } catch (err) {
      output = { cmd: c, text: t(err.message), error: true };
    } finally {
      busy = false;
    }
  }
  const quick = q => { cmd = q; send(); };
  function reuse(c) {
    cmd = c;
    scrollTo({ top: 0, behavior: 'smooth' });
    say(t('Commande reprise : {c}', { c }));
  }
</script>

<BackTitle title={t('Console')} sub={srv ? clean(srv.name) : ''} />

{#if !view}
  <p class="muted">{t('Il faut la permission « rcon » ou l\'accès aux données privées sur ce serveur.')}</p>
{:else}
  {#if canLogs && canRcon}
    <div class="tabs"><Seg label={t('Console')} value={view} onchange={v => (tab = v)} items={[['game', t('Console du jeu')], ['rcon', 'RCON']]} /></div>
  {/if}

  {#if view === 'game'}
    <section class="card box">
      {#if lines === null}
        <p class="muted">{t('Chargement…')}</p>
      {:else if !logsOn}
        <p class="muted">{t('Logs non configurés : ajoute la source des logs (FTP ou dossier) dans les réglages du serveur.')}</p>
      {:else}
        <div class="bar">
          <label class="sr-only" for="log-filter">{t('Filtrer la console')}</label>
          <input id="log-filter" bind:value={filter} placeholder={t('Filtrer la console…')} autocomplete="off" autocapitalize="none" spellcheck="false" />
          <label class="check"><input type="checkbox" bind:checked={follow} />{t('Défilement auto')}</label>
        </div>
        <div class="console" bind:this={box} aria-live="off">
          {#each shown as l, i (i)}
            <div class="ln" class:chat={/\bsay(team)?\b/.test(l)}>{#each colorParts(l) as p, j (j)}<span style:color={p.color}>{p.text}</span>{/each}</div>
          {:else}
            <p class="muted">{filter ? t('Aucune ligne ne correspond.') : t('Aucune ligne pour le moment.')}</p>
          {/each}
        </div>
        <p class="muted hint">{t('Mise à jour toutes les 4 secondes. Les lignes de chat sont surlignées.')}</p>
      {/if}
    </section>
  {:else if !rconOn}
    <p class="muted">{t('Pas de mot de passe rcon enregistré pour ce serveur (Réglages du serveur).')}</p>
  {:else}
    <form class="card box" onsubmit={send}>
      <label class="sr-only" for="rcon-cmd">{t('Commande')}</label>
      <div class="line">
        <input id="rcon-cmd" bind:value={cmd} autocomplete="off" autocapitalize="none" spellcheck="false" placeholder={t('ex. : status')} />
        <button class="btn primary" disabled={busy || !cmd.trim()}>{t('Envoyer')}</button>
      </div>
      <div class="quick">{#each QUICK as q (q)}<button type="button" onclick={() => quick(q)}>{q}</button>{/each}</div>
      {#if output}
        <div class="out" class:err={output.error}><span class="muted">&gt; {output.cmd}</span><pre>{output.text}</pre></div>
      {/if}
    </form>

    <section aria-label={t('Historique')}>
      <h2 class="label sec">{t('Dernières commandes')}</h2>
      <div class="card rows">
        {#each log.slice(0, 60) as l, i (i)}
          <div class="item">
            <button class="row" aria-expanded={open === i} onclick={() => (open = open === i ? -1 : i)}>
              <code>{l.command}</code>
              <span class="muted when">{#if l.error}<span class="bad">{t('erreur')}</span>{' · '}{/if}{agoText(l.ts)}</span>
            </button>
            {#if open === i}
              <div class="detail">
                {#if l.error}<pre class="bad">{l.error}</pre>{:else}<pre>{clean(l.output || '').trim().slice(0, 200) || '(ok)'}</pre>{/if}
                <div class="dacts"><span class="muted">{dt(l.ts)}</span><button class="btn" onclick={() => reuse(l.command)}>{t('Reprendre')}</button></div>
              </div>
            {/if}
          </div>
        {:else}
          <p class="muted empty">{t('Aucune commande envoyée.')}</p>
        {/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  .tabs { margin-bottom: 12px; max-width: 420px; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
  .box p { margin: 0; }
  .bar { display: flex; flex-wrap: wrap; gap: 8px 14px; align-items: center; }
  .bar input:not([type]) { flex: 1 1 220px; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .check { display: inline-flex; align-items: center; gap: 8px; font-size: 14px; color: var(--text-2); font-weight: 700; }
  .check input { width: 18px; height: 18px; accent-color: var(--accent); }
  .console { height: min(60vh, 520px); overflow: auto; border-radius: 12px; background: #0A0D12; border: 1px solid var(--line); padding: 8px 10px; font-family: ui-monospace, monospace; font-size: 12px; line-height: 1.5; }
  .ln { white-space: pre-wrap; word-break: break-word; color: var(--text-2); }
  .ln.chat { background: var(--accent-soft); border-radius: 4px; padding: 0 4px; margin: 0 -4px; }
  .hint { font-size: 12px; }
  .line { display: flex; gap: 8px; }
  .line input { flex: 1; min-width: 0; height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; font-family: ui-monospace, monospace; }
  .quick { display: flex; flex-wrap: wrap; gap: 6px; }
  .quick button { height: 32px; padding: 0 10px; border-radius: 9px; border: 1px solid var(--line); background: transparent; color: var(--text-2); font-family: ui-monospace, monospace; font-size: 12px; }
  .out { border-radius: 12px; background: #0A0D12; border: 1px solid var(--line); padding: 10px 12px; max-height: 50vh; overflow: auto; }
  .out.err pre { color: var(--danger-text); }
  pre { margin: 6px 0 0; font-size: 12px; line-height: 1.45; white-space: pre-wrap; word-break: break-word; }
  .sec { display: block; margin: 14px 2px 8px; }
  .rows { overflow: hidden; }
  .item { border-top: 1px solid var(--line-soft); }
  .item:first-child { border-top: 0; }
  .row { width: 100%; display: flex; align-items: center; justify-content: space-between; gap: 10px; min-height: 44px; padding: 8px 14px; border: 0; background: transparent; color: var(--text); text-align: left; }
  .row code { font-size: 13px; overflow-wrap: anywhere; }
  .when { font-size: 12px; white-space: nowrap; }
  .bad { color: var(--danger-text); }
  .detail { padding: 0 14px 12px; }
  .detail pre { margin: 0; border-radius: 10px; background: #0A0D12; border: 1px solid var(--line); padding: 8px 10px; }
  .dacts { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-top: 8px; font-size: 12px; }
  .dacts .btn { min-height: 36px; padding: 0 12px; font-size: 13px; }
  .empty { margin: 0; padding: 14px; }
</style>
