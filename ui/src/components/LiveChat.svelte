<script>
  // The in-game chat (private data), newest first, with a box to write to every player.
  import CodName from './CodName.svelte';
  import { sayAll } from '../lib/actions.js';
  import { ago } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ messages: any[], now: number, canSay: boolean, serverId: number, onsent: () => void }} */
  let { messages, now, canSay, serverId, onsent } = $props();
  let text = $state('');
  let busy = $state(false);

  async function send(e) {
    e.preventDefault();
    if (!text.trim()) return;
    busy = true;
    if (await sayAll(serverId, text)) { text = ''; onsent(); }
    busy = false;
  }
</script>

<section class="card chat" aria-label={t('Chat')}>
  {#if canSay}
    <form onsubmit={send}>
      <label class="in"><span class="sr-only">{t('Message à tous les joueurs')}</span>
        <input bind:value={text} maxlength="200" placeholder={t('Écrire à tous les joueurs…')} />
      </label>
      <button class="btn primary" disabled={busy || !text.trim()}>{t('Envoyer')}</button>
    </form>
  {/if}
  {#each messages as m, i (i)}
    <div class="msg" class:cmd={m.message?.startsWith('!')}>
      <div class="meta">
        <span class="who"><CodName raw={m.name_raw} /></span>
        {#if m.team}<span class="team">{t('ÉQUIPE')}</span>{/if}
        <span class="when">{ago(m.ts, now)}</span>
      </div>
      <div class="text">{m.message}</div>
    </div>
  {:else}
    <p class="empty">{t('Aucun message récent.')}</p>
  {/each}
</section>

<style>
  .chat { overflow: hidden; }
  form { display: flex; gap: 8px; padding: 10px; border-bottom: 1px solid var(--line); background: #11161E; }
  .in { flex: 1; display: flex; }
  input { flex: 1; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); padding: 0 12px; font-size: 16px; }
  form .btn { min-height: 44px; }
  .msg { padding: 10px 14px; border-top: 1px solid var(--line-soft); }
  form + .msg, .msg:first-child { border-top: 0; }
  .meta { display: flex; align-items: center; gap: 8px; font-size: 12px; color: var(--muted); }
  .who { display: flex; min-width: 0; font-weight: 800; font-size: 13px; }
  .team { font-size: 10px; font-weight: 800; padding: 1px 5px; border-radius: 5px; background: var(--line); color: var(--text-2); }
  .when { margin-left: auto; flex: none; }
  .text { font-size: 14px; margin-top: 2px; overflow-wrap: anywhere; }
  .cmd .text { color: var(--muted); }
  .empty { margin: 0; padding: 18px 16px; font-size: 14px; color: var(--muted); }
</style>
