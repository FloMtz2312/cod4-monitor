<script>
  // XP boost of pm_sniper (dvar xp_scale): applies at the next XP gain, back to the server.cfg value when the server restarts.
  import Sheet from './Sheet.svelte';
  import { rcon } from '../lib/actions.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void, serverId: number, current: string, ondone: () => void }} */
  let { open, onclose, serverId, current, ondone } = $props();
  let custom = $state('');
  let busy = $state(false);

  $effect(() => { if (open) custom = ''; });
  const value = $derived(parseFloat(custom.replace(',', '.')));
  const valid = $derived(custom.trim() !== '' && Number.isFinite(value) && value >= 0 && value <= 1000);

  async function set(v) {
    busy = true;
    const ok = await rcon(serverId, `xp_scale ${v}`, t('Boost XP réglé sur x{n}', { n: v }));
    busy = false;
    if (ok) { ondone(); onclose(); }
  }
</script>

<Sheet {open} {onclose} label={t('Boost XP')}>
  <h2>{t('Boost XP')}</h2>
  <p class="muted">{t('Multiplie l\'XP gagnée par les joueurs (xp_scale). Effet au prochain gain d\'XP ; la valeur du server.cfg revient au redémarrage du serveur.')}</p>
  <p class="cur">{t('Actuel : x{n}', { n: current || '?' })}</p>
  <div class="presets">
    {#each ['1', '2', '3', '5', '10'] as v (v)}
      <button class="btn" class:on={current === v} disabled={busy} onclick={() => set(v)}>x{v}</button>
    {/each}
  </div>
  <form class="custom" onsubmit={e => { e.preventDefault(); if (valid) set(String(value)); }}>
    <label>
      <span class="sr-only">{t('Autre valeur')}</span>
      <input bind:value={custom} inputmode="decimal" maxlength="6" placeholder={t('Autre valeur (ex. 1.5)')} autocomplete="off" />
    </label>
    <button class="btn primary" disabled={busy || !valid}>{t('Appliquer')}</button>
  </form>
</Sheet>

<style>
  h2 { font-size: 18px; }
  p { margin: 4px 0 12px; font-size: 13px; }
  .cur { font-size: 15px; font-weight: 800; color: var(--accent); }
  .presets { display: grid; grid-template-columns: repeat(5, minmax(0, 1fr)); gap: 8px; margin-bottom: 10px; }
  .presets .btn { padding: 0; }
  .presets .btn.on { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .custom { display: flex; gap: 8px; }
  .custom label { flex: 1; min-width: 0; }
  .custom input { width: 100%; height: 46px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
</style>
