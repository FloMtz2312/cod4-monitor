<script>
  import Sheet from './Sheet.svelte';
  import ServerRow from './ServerRow.svelte';
  import { app, selectServer } from '../lib/state.svelte.js';
  import { go } from '../lib/router.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void }} */
  let { open, onclose } = $props();
</script>

<Sheet {open} {onclose} label={t('Changer de serveur')}>
  <h2>{t('Serveurs')}</h2>
  <div class="list">
    {#each app.servers as s (s.id)}
      <ServerRow server={s} selected={s.id === app.serverId} onpick={() => { selectServer(s.id); onclose(); }} />
    {/each}
  </div>
  <button class="btn ghost all" onclick={() => { onclose(); go('serveurs'); }}>{t('Voir tous les serveurs')}</button>
</Sheet>

<style>
  h2 { font-size: 18px; margin-bottom: 12px; }
  .list { display: flex; flex-direction: column; gap: 8px; }
  .all { width: 100%; margin-top: 8px; color: var(--accent); }
</style>
