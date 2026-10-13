<script>
  import Sheet from './Sheet.svelte';
  import { restart, nextMap, mapRestart } from '../lib/actions.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void, serverId: number, mapTitle: string, canRcon?: boolean }} */
  let { open, onclose, serverId, mapTitle, canRcon = false } = $props();
  let busy = $state(false);

  async function act(fn) {
    busy = true;
    const ok = await fn(serverId);
    busy = false;
    if (ok) onclose();
  }
</script>

<Sheet {open} {onclose} label={t('Relancer la partie')}>
  <h2>{t('Relancer la partie')}</h2>
  <p class="muted">{t('Les scores repartent à zéro.')}</p>
  <div class="acts">
    <button class="btn primary" disabled={busy} onclick={() => act(restart)}>{t('Relancer sur {map}', { map: mapTitle })}</button>
    {#if canRcon}
      <button class="btn" disabled={busy} onclick={() => act(mapRestart)}>{t('Recharger la map (map_restart)')}</button>
    {/if}
    <button class="btn" disabled={busy} onclick={() => act(nextMap)}>{t('Passer à la map suivante')}</button>
    <button class="btn ghost" onclick={onclose}>{t('Annuler')}</button>
  </div>
</Sheet>

<style>
  h2 { font-size: 18px; }
  p { margin: 4px 0 14px; font-size: 13px; }
  .acts { display: flex; flex-direction: column; gap: 8px; }
</style>
