<script>
  // Démos (#/demos, #/demos/<player key>; private data): the demos recorded on the server, one per player and per
  // map, seen from the player's eyes. Pick one in the list (or the quick picker) to open it in the demo viewer,
  // which reads the file straight from the game server: nothing to download or import.
  import BackTitle from '../components/BackTitle.svelte';
  import Seg from '../components/Seg.svelte';
  import CodName from '../components/CodName.svelte';
  import Icon from '../components/Icon.svelte';
  import { get, post } from '../lib/api.js';
  import { app } from '../lib/state.svelte.js';
  import { permsOf, isOwner } from '../lib/perms.js';
  import { route, href } from '../lib/router.svelte.js';
  import { clean } from '../lib/format.js';
  import { dt } from '../lib/moderation.js';
  import { size, length, live, viewerURL, fileURL, viewerInstalled } from '../lib/demos.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const allowed = $derived(permsOf(app.me, id).has('private'));
  const owner = $derived(isOwner(app.me, id));
  const player = $derived(route.params[0] ?? '');

  let demos = $state(null);
  let viewer = $state(true);
  let filter = $state('');
  let show = $state('all'); // all | live | kept
  let pick = $state('');
  let busy = $state(0);

  $effect(() => {
    const sid = id, p = player;
    if (!sid || !allowed) return;
    let alive = true;
    demos = null;
    get(`/api/servers/${sid}/demos?limit=500${p ? '&player=' + encodeURIComponent(p) : ''}`)
      .then(r => { if (alive) demos = r.demos ?? []; }).catch(e => { if (alive) { demos = []; say(e.message); } });
    return () => { alive = false; };
  });
  viewerInstalled().then(v => { viewer = v; });

  const name = d => d.name_raw || d.player || d.file;
  const shown = $derived((demos ?? []).filter(d => {
    if (show === 'live' && !live(d)) return false;
    if (show === 'kept' && !d.kept) return false;
    const q = filter.trim().toLowerCase();
    return !q || clean(name(d)).toLowerCase().includes(q) || d.file.toLowerCase().includes(q);
  }));

  function open(d) {
    if (!d) return;
    window.open(viewerURL(id, d), '_blank', 'noopener');
  }

  async function keep(d, on) {
    busy = d.id;
    try {
      await post(`/api/servers/${id}/demos/${d.id}/keep`, { keep: on });
      d.kept = on ? 2 : 0;
      d.kept_reason = on ? 'manual' : '';
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    } finally {
      busy = 0;
    }
  }

  const REASON = { manual: 'gardée à la main', report: 'signalement', suspect: 'stats suspectes', watch: 'joueur surveillé', evasion: 'contournement de ban', b3ban: 'ban B3', ban: 'ban' };
</script>

<BackTitle title={t('Démos')} sub={t('Une démo par joueur et par map, vue par ses yeux.')} fallback="moderation" />

{#if !allowed}
  <p class="muted">{t('Réservé à l\'équipe ayant accès aux données privées.')}</p>
{:else}
  {#if !viewer}
    <p class="note">{t('Le lecteur de démos n\'est pas installé sur ce monitor (il est inclus dans l\'image Docker) : les démos peuvent seulement être téléchargées.')}</p>
  {/if}
  {#if player}
    <p class="muted who">{t('Démos d\'un seul joueur.')} <a href={href('demos')}>{t('Voir toutes les démos')}</a></p>
  {/if}

  {#if demos === null}
    <p class="muted">{t('Chargement…')}</p>
  {:else if !demos.length}
    <div class="card box">
      <p class="muted">{t('Aucune démo pour l\'instant.')}</p>
      {#if owner}<p class="muted small">{t('Active l\'enregistrement automatique dans les réglages du serveur (Plus, Réglages du serveur, Démos).')}</p>
        <a class="btn" href={href('reglages', String(id))}><Icon name="settings" size={18} />{t('Réglages du serveur')}</a>{/if}
    </div>
  {:else}
    {#if viewer}
      <div class="card quick">
        <label class="sr-only" for="demo-pick">{t('Ouvrir une démo')}</label>
        <select id="demo-pick" bind:value={pick}>
          <option value="">{t('Ouvrir une démo…')}</option>
          {#each shown.slice(0, 200) as d (d.id)}
            <option value={String(d.id)}>{clean(name(d))} · {dt(d.started_at)} · {length(d)}{live(d) ? ' · ' + t('en cours') : ''}</option>
          {/each}
        </select>
        <button class="btn primary" disabled={!pick} onclick={() => open(demos.find(d => String(d.id) === pick))}><Icon name="play" size={18} />{t('Ouvrir')}</button>
      </div>
    {/if}

    <div class="tools">
      <input class="search" bind:value={filter} placeholder={t('Joueur ou fichier…')} aria-label={t('Rechercher une démo')} autocomplete="off" spellcheck="false" />
      <Seg value={show} onchange={v => (show = v)} label={t('Filtrer les démos')} items={[['all', t('Toutes')], ['live', t('En cours')], ['kept', t('Gardées')]]} />
    </div>
    <p class="muted small count">{t('{n} démos', { n: shown.length })} · {size(shown.reduce((s, d) => s + d.size, 0))}</p>

    <div class="card list">
      {#each shown as d (d.id)}
        <div class="row">
          <span class="ic" class:rec={live(d)}><Icon name="film" size={20} /></span>
          <span class="main">
            <a class="pn" href={href('joueur', d.player)}><CodName raw={name(d)} /></a>
            <small>{dt(d.started_at)} → {live(d) ? t('en cours') : dt(d.last_write)} · {length(d)} · {size(d.size)}</small>
            {#if live(d) || d.kept}
              <span class="badges">
                {#if live(d)}<span class="b rec">{t('Enregistrement en cours')}</span>{/if}
                {#if d.kept}<span class="b kept">{t('Gardée : {why}', { why: t(REASON[d.kept_reason] ?? d.kept_reason ?? '') })}</span>{/if}
              </span>
            {/if}
          </span>
          <span class="acts">
            {#if viewer}<a class="btn primary sm" href={viewerURL(id, d)} target="_blank" rel="noopener"><Icon name="play" size={16} />{t('Voir')}</a>{/if}
            <a class="btn sm icon" href={fileURL(id, d)} download={d.file} aria-label={t('Télécharger')} title={t('Télécharger')}><Icon name="download" size={16} /></a>
            {#if d.kept === 2}
              <button class="btn sm" disabled={busy === d.id} onclick={() => keep(d, false)}>{t('Ne plus garder')}</button>
            {:else}
              <button class="btn sm" disabled={busy === d.id} onclick={() => keep(d, true)} title={t('Garder cette démo au-delà de la durée de conservation')}>{t('Garder')}</button>
            {/if}
          </span>
        </div>
      {:else}
        <p class="muted none">{t('Aucune démo ne correspond.')}</p>
      {/each}
    </div>
  {/if}
{/if}

<style>
  .note { padding: 12px 14px; border-radius: 12px; background: var(--surface-2); font-size: 13px; color: var(--text-2); margin: 0 0 12px; }
  .who { margin: 0 0 10px; font-size: 13px; }
  .box { padding: 16px; display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
  .box p { margin: 0; }
  .small { font-size: 12px; }
  .quick { display: flex; gap: 8px; padding: 10px; margin-bottom: 12px; }
  .quick select { flex: 1; min-width: 0; height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text); padding: 0 10px; font-size: 15px; }
  .tools { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  .search { flex: 1 1 220px; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text); padding: 0 12px; font-size: 16px; }
  .count { margin: 8px 2px; }
  .list { overflow: hidden; }
  .row { display: flex; align-items: center; gap: 12px; padding: 12px 14px; border-top: 1px solid var(--line-soft); }
  .row:first-child { border-top: 0; }
  .ic { width: 38px; height: 38px; flex: none; border-radius: 12px; background: var(--surface-2); color: var(--muted); display: flex; align-items: center; justify-content: center; }
  .ic.rec { color: var(--danger); background: rgba(255, 99, 105, .12); }
  .main { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .pn { font-weight: 800; color: var(--text); text-decoration: none; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .main small { color: var(--muted); font-size: 12px; }
  .badges { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 2px; }
  .b { font-size: 11px; font-weight: 800; padding: 2px 8px; border-radius: 8px; }
  .b.rec { background: rgba(255, 99, 105, .14); color: var(--danger-text); }
  .b.kept { background: var(--accent-soft); color: var(--accent); }
  .acts { display: flex; gap: 6px; flex: none; }
  .sm { height: 36px; padding: 0 12px; font-size: 13px; gap: 6px; }
  .icon { width: 36px; padding: 0; justify-content: center; }
  .none { padding: 14px; margin: 0; }
  @media (max-width: 560px) {
    .row { flex-wrap: wrap; }
    .acts { width: 100%; padding-left: 50px; }
  }
</style>
