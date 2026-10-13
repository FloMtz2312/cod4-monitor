<script>
  // A message to all players (say) or to one (tell).
  import Sheet from './Sheet.svelte';
  import { sayAll, tell } from '../lib/actions.js';
  import { clean } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void, serverId: number, target: any }} */
  let { open, onclose, serverId, target } = $props();
  let text = $state('');
  let busy = $state(false);

  $effect(() => { if (open) text = ''; });

  async function send(e) {
    e.preventDefault();
    if (!text.trim()) return;
    busy = true;
    const ok = target ? await tell(serverId, target.name_raw, text) : await sayAll(serverId, text);
    busy = false;
    if (ok) onclose();
  }
</script>

<Sheet {open} {onclose} label={t('Message')}>
  <form onsubmit={send}>
    <h2>{target ? t('Message privé à {name}', { name: clean(target.name_raw) }) : t('Message à tous les joueurs')}</h2>
    <label>
      <span class="sr-only">{t('Message')}</span>
      <textarea bind:value={text} rows="3" maxlength="200" placeholder={t('Ex. : rejoignez le Discord !')}></textarea>
    </label>
    <button class="btn primary" disabled={busy || !text.trim()}>{t('Envoyer')}</button>
  </form>
</Sheet>

<style>
  form { display: flex; flex-direction: column; gap: 12px; }
  h2 { font-size: 18px; }
  textarea { width: 100%; border-radius: 14px; border: 1px solid var(--line); background: var(--surface-2); padding: 12px 14px; font-size: 16px; resize: none; }
</style>
