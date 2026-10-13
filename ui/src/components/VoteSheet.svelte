<script>
  // Maps of the end-of-match vote (pm_sniper's scr_mapvote_maps): a checklist over the server's maps. Applied at once;
  // the server.cfg list comes back when the server restarts.
  import Sheet from './Sheet.svelte';
  import { get } from '../lib/api.js';
  import { rcon } from '../lib/actions.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void, serverId: number }} */
  let { open, onclose, serverId } = $props();

  /** @type {{name: string, title: string, custom: boolean}[] | null} */
  let maps = $state(null);
  /** @type {string[]} */
  let orig = $state([]);
  let sel = $state(new Set());
  let failed = $state('');
  let confirming = $state(false);
  let busy = $state(false);

  $effect(() => {
    if (!open) return;
    maps = null;
    failed = '';
    confirming = false;
    const sid = serverId;
    Promise.all([get(`/api/servers/${sid}/catalog`), get(`/api/servers/${sid}/dvar?name=scr_mapvote_maps`)])
      .then(([c, d]) => {
        orig = String(d.value ?? '').split(/\s+/).filter(Boolean);
        sel = new Set(orig);
        maps = c.maps ?? [];
      })
      .catch(e => { failed = e.message; });
  });

  const known = $derived(new Set((maps ?? []).map(m => m.name)));
  const missing = $derived(orig.filter(m => maps && !known.has(m)));
  // the server's order for the maps already in the vote, new ones at the end; unknown maps dropped
  const list = $derived(maps ? [...orig.filter(m => sel.has(m)), ...maps.map(m => m.name).filter(m => sel.has(m) && !orig.includes(m))].filter(m => known.has(m)) : []);

  function toggle(name) {
    const s = new Set(sel);
    if (s.has(name)) s.delete(name); else s.add(name);
    sel = s;
    confirming = false;
  }
  const all = on => { sel = new Set(on ? maps.map(m => m.name) : []); confirming = false; };

  async function save() {
    busy = true;
    const ok = await rcon(serverId, `scr_mapvote_maps "${list.join(' ')}"`, t('Vote : {n} maps enregistrées', { n: list.length }));
    busy = false;
    if (ok) onclose();
  }
</script>

<Sheet {open} {onclose} label={t('Maps du vote de fin de match')}>
  <h2>{t('Maps du vote de fin de match')}</h2>
  <p class="muted">{t('Le vote propose 5 maps tirées au hasard parmi celles cochées (map en cours exclue), en S&D ou FFA. Les maps doivent avoir des spawns S&D et FFA.')}</p>
  {#if failed}
    <p class="warn">{failed}</p>
  {:else if !maps}
    <p class="muted">{t('Chargement…')}</p>
  {:else}
    {#if missing.length}
      <p class="warn">{t('Dans le vote mais absentes du serveur (le vote échouerait sur elles) : {m}. Décochées si tu enregistres.', { m: missing.join(', ') })}</p>
    {/if}
    <div class="row">
      <button class="btn ghost sm" onclick={() => all(true)}>{t('Tout cocher')}</button>
      <button class="btn ghost sm" onclick={() => all(false)}>{t('Tout décocher')}</button>
    </div>
    <ul class="list">
      {#each maps as m (m.name)}
        <li>
          <label class:on={sel.has(m.name)}>
            <input type="checkbox" checked={sel.has(m.name)} onchange={() => toggle(m.name)} />
            <span class="t">{m.title}</span>
            <span class="s">{m.custom ? t('custom') : m.name}</span>
          </label>
        </li>
      {/each}
    </ul>
    <div class="bar">
      <p class="count"><b>{list.length}</b> {t('map(s) cochée(s)')}{#if list.length < 2}<span class="ko">{' — '}{t('il en faut au moins 2 pour le vote')}</span>{/if}</p>
      {#if confirming}
        <div class="two">
          <button class="btn ghost" onclick={() => (confirming = false)}>{t('Annuler')}</button>
          <button class="btn primary" disabled={busy} onclick={save}>{t('Enregistrer {n} maps', { n: list.length })}</button>
        </div>
      {:else}
        <button class="btn primary" disabled={list.length < 2} onclick={() => (confirming = true)}>{t('Enregistrer')}</button>
      {/if}
      <p class="muted small">{t('Appliqué tout de suite au serveur (scr_mapvote_maps) ; au redémarrage du serveur, c\'est la liste du server.cfg qui revient.')}</p>
    </div>
  {/if}
</Sheet>

<style>
  h2 { font-size: 18px; }
  p { margin: 4px 0 10px; font-size: 13px; }
  .warn { color: var(--warn); }
  .row { display: flex; gap: 8px; margin-bottom: 6px; }
  .sm { min-height: 36px; padding: 0 10px; font-size: 13px; }
  .list { list-style: none; margin: 0; padding: 0; display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 6px; }
  label { display: grid; grid-template-columns: auto minmax(0, 1fr); grid-template-rows: auto auto; column-gap: 8px; align-items: center; padding: 8px 10px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); cursor: pointer; min-height: 52px; }
  label.on { border-color: var(--accent); background: var(--accent-soft); }
  input { grid-row: 1 / 3; width: 18px; height: 18px; accent-color: var(--accent); margin: 0; }
  .t { font-weight: 800; font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .s { font-size: 12px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .bar { position: sticky; bottom: -1px; padding-top: 10px; background: var(--surface); display: flex; flex-direction: column; gap: 8px; }
  .bar p { margin: 0; }
  .bar > .btn { width: 100%; }
  .count { font-size: 14px; }
  .ko { color: var(--danger-text); }
  .small { font-size: 12px; }
  .two { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  @media (min-width: 960px) { .list { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
