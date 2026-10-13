<script>
  import ServerLogo from './ServerLogo.svelte';
  import { mapName } from '../lib/state.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ server: any, selected?: boolean, onpick: () => void }} */
  let { server, selected = false, onpick } = $props();
  const name = $derived(server.name.replace(/\^\d/g, ''));
</script>

<button class="row" class:selected aria-pressed={selected} onclick={onpick}>
  <ServerLogo {server} size={42} muted={!selected && !server.accent && !server.logo_ts} />
  <span class="txt">
    <span class="name">{name}</span>
    <span class="st"><span class="dot" class:on={server.online}></span>
      {server.online ? `${mapName(server.map)}${server.gametype ? ' · ' + server.gametype.toUpperCase() : ''}` : t('Hors ligne')}</span>
  </span>
  <span class="count">{server.online ? `${server.players ?? 0}/${server.max_players ?? '?'}` : '—'}</span>
</button>

<style>
  .row {
    width: 100%; display: flex; align-items: center; gap: 12px; padding: 12px; border-radius: 16px;
    border: 1px solid var(--line); background: var(--surface-2); text-align: left;
  }
  .row.selected { border-color: var(--accent); }
  .txt { flex: 1; min-width: 0; }
  .name { display: block; font-weight: 800; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .st { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--text-2); }
  .dot { width: 7px; height: 7px; border-radius: 4px; background: var(--danger); }
  .dot.on { background: var(--ok); }
  .count { font-family: var(--font-display); font-weight: 700; font-size: 18px; }
</style>
