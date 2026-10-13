<script>
  // Map change: the server's maps (custom ones from the FTP listing, else the stock maps) with their image,
  // a search and a filter, the game mode (rcon permission), the mod (read-only), then a confirmation.
  import Sheet from './Sheet.svelte';
  import Icon from './Icon.svelte';
  import { get } from '../lib/api.js';
  import { changeMap, changeMapMode } from '../lib/actions.js';
  import { mapName } from '../lib/state.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void, serverId: number, current: string, gametype?: string, mod?: string, players?: number, canMode?: boolean }} */
  let { open, onclose, serverId, current, gametype = '', mod = '', players = 0, canMode = false } = $props();
  const STOCK = ['mp_backlot', 'mp_bloc', 'mp_bog', 'mp_cargoship', 'mp_citystreets', 'mp_convoy', 'mp_countdown', 'mp_crash',
    'mp_crossfire', 'mp_farm', 'mp_overgrown', 'mp_pipeline', 'mp_shipment', 'mp_showdown', 'mp_strike', 'mp_vacant'];
  const MODES = [['sd', 'Recherche et destruction'], ['war', 'Match à mort par équipe'], ['dm', 'Chacun pour soi'],
    ['dom', 'Domination'], ['koth', 'Quartier général'], ['sab', 'Sabotage']];

  /** @type {{name: string, title: string, custom: boolean, image?: string}[]} */
  let maps = $state(STOCK.map(name => ({ name, title: mapName(name), custom: false })));
  let picked = $state('');
  let q = $state('');
  let filter = $state('all');
  let mode = $state('sd');
  let confirming = $state(false);
  let busy = $state(false);
  let failed = $state('');
  /** @type {Record<string, boolean>} */
  let noImg = $state({});

  $effect(() => {
    if (!open) return;
    picked = '';
    q = '';
    confirming = false;
    failed = '';
    mode = MODES.some(([k]) => k === gametype) ? gametype : 'sd';
    get(`/api/servers/${serverId}/catalog`).then(r => {
      if (r.maps?.length) maps = r.maps;
      failed = r.error ?? '';
    }).catch(() => {});
  });

  const shown = $derived(maps.filter(m => (filter === 'all' || (filter === 'custom') === !!m.custom)
    && (m.name + ' ' + m.title).toLowerCase().includes(q.trim().toLowerCase())));
  const sel = $derived(maps.find(x => x.name === picked));
  const modeLabel = $derived(t(MODES.find(([k]) => k === mode)?.[1] ?? mode));

  async function go() {
    if (!sel) return;
    busy = true;
    const ok = canMode && mode !== gametype
      ? await changeMapMode(serverId, sel.name, sel.title, mode)
      : await changeMap(serverId, sel.name, sel.title);
    busy = false;
    if (ok) onclose();
  }
</script>

<Sheet {open} {onclose} label={t('Changer de map')}>
  <h2>{t('Changer de map')}</h2>
  <p class="muted">{t('La partie en cours s\'arrête pour tous les joueurs.')}</p>
  {#if failed}<p class="warn">{failed}</p>{/if}
  <div class="tools">
    <label class="search">
      <Icon name="search" size={18} />
      <span class="sr-only">{t('Rechercher une map')}</span>
      <input type="search" bind:value={q} placeholder={t('Rechercher une map…')} autocomplete="off" />
    </label>
    <div class="filters" role="radiogroup" aria-label={t('Filtre')}>
      {#each [['all', 'Toutes'], ['custom', 'Custom'], ['stock', 'Stock']] as [k, label] (k)}
        <button type="button" role="radio" aria-checked={filter === k} class:on={filter === k} onclick={() => (filter = k)}>{t(label)}</button>
      {/each}
    </div>
  </div>
  <div class="grid">
    {#each shown as m (m.name)}
      <button class="map" class:cur={m.name === current} class:on={m.name === picked} aria-pressed={m.name === picked} onclick={() => { picked = m.name; confirming = false; }}>
        <span class="img">
          {#if m.image && !noImg[m.name]}<img src={'/' + m.image} alt="" loading="lazy" onerror={() => (noImg[m.name] = true)} />{/if}
          {#if m.name === current}<span class="tag cur">{t('en cours')}</span>{:else if m.custom}<span class="tag">{t('custom')}</span>{/if}
        </span>
        <span class="t">{m.title}</span>
        <span class="s">{m.name}</span>
      </button>
    {:else}
      <p class="muted none">{t('Aucune map ne correspond')}</p>
    {/each}
  </div>
  <div class="bar">
    {#if canMode}
      <label class="field">
        <span>{t('Mode')}</span>
        <select bind:value={mode}>
          {#each MODES as [k, label] (k)}<option value={k}>{t(label)} ({k})</option>{/each}
        </select>
      </label>
    {/if}
    {#if mod}
      <p class="mod muted">{t('Mod : {m} (fixé au démarrage du serveur)', { m: mod })}</p>
    {/if}
    {#if confirming && sel}
      <p class="confirm">
        {t('Lancer {map} en {mode} ?', { map: sel.title, mode: modeLabel })}
        {#if players > 0}<br /><b>{t('{n} joueur(s) connecté(s) : la partie en cours sera interrompue.', { n: players })}</b>{/if}
      </p>
      <div class="two">
        <button class="btn ghost" onclick={() => (confirming = false)}>{t('Annuler')}</button>
        <button class="btn primary" disabled={busy} onclick={go}>{t('Confirmer')}</button>
      </div>
    {:else}
      <button class="btn primary" disabled={!sel} onclick={() => (confirming = true)}>
        {sel ? t('Lancer {map}', { map: sel.title }) : t('Choisis une map')}
      </button>
    {/if}
  </div>
</Sheet>

<style>
  h2 { font-size: 18px; }
  p { margin: 4px 0 12px; font-size: 13px; }
  .warn { color: var(--warn); }
  .tools { display: flex; flex-direction: column; gap: 8px; margin-bottom: 10px; }
  .search { display: flex; align-items: center; gap: 8px; height: 44px; padding: 0 12px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); color: var(--muted); }
  .search input { flex: 1; min-width: 0; border: 0; background: transparent; outline: none; font-size: 16px; color: var(--text); }
  .filters { display: flex; gap: 6px; }
  .filters button { flex: 1; height: 36px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); font-weight: 700; font-size: 13px; }
  .filters button.on { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .map { border-radius: 14px; border: 1px solid var(--line); background: var(--surface-2); text-align: left; padding: 0 0 8px; overflow: hidden; min-width: 0; }
  .map.cur { border-color: #3A4A63; }
  .map.on { border-color: var(--accent); background: var(--accent-soft); box-shadow: 0 0 0 1px var(--accent); }
  .img { position: relative; display: block; aspect-ratio: 16 / 9; background: var(--line-soft); margin-bottom: 6px; }
  .img img { width: 100%; height: 100%; object-fit: cover; display: block; }
  .tag { position: absolute; top: 6px; left: 6px; font-size: 11px; font-weight: 800; padding: 2px 7px; border-radius: 7px; background: rgba(11, 14, 19, .8); color: var(--text-2); }
  .tag.cur { color: var(--accent); }
  .t { display: block; font-weight: 800; padding: 0 10px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .s { display: block; font-size: 12px; color: var(--muted); padding: 0 10px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .none { grid-column: 1 / -1; }
  .bar { position: sticky; bottom: -1px; padding-top: 12px; background: var(--surface); display: flex; flex-direction: column; gap: 8px; }
  .bar .btn { width: 100%; }
  .field { display: flex; align-items: center; gap: 10px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .field select { flex: 1; min-width: 0; height: 42px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 10px; font-size: 16px; }
  .mod { margin: 0; font-size: 12px; }
  .confirm { margin: 0; font-size: 14px; }
  .confirm b { color: var(--warn); font-weight: 700; }
  .two { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  @media (min-width: 960px) { .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
