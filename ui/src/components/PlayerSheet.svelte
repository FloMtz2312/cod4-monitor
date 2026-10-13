<script>
  // Actions on a connected player: profile, private message, kick, bans.
  import Sheet from './Sheet.svelte';
  import CodName from './CodName.svelte';
  import { kick, ban } from '../lib/actions.js';
  import { teamName, pingColor } from '../lib/format.js';
  import { href } from '../lib/router.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void, player: any, serverId: number, perms: Set<string>, onmessage: (p: any) => void }} */
  let { open, onclose, player, serverId, perms, onmessage } = $props();
  let reason = $state('');
  let custom = $state('');
  let confirmPerm = $state(false);
  let busy = $state(false);

  $effect(() => { if (open) { reason = ''; custom = ''; confirmPerm = false; } });
  // free duration: 30m, 12h, 7d (the server's tempban format)
  const customOk = $derived(/^\d{1,4}[mhd]$/.test(custom.trim().toLowerCase()));

  async function act(fn) {
    busy = true;
    const ok = await fn();
    busy = false;
    if (ok) onclose();
  }
</script>

<Sheet {open} {onclose} label={t('Actions sur le joueur')}>
  {#if player}
    <div class="who">
      <span class="avatar">{(player.name_clean || '?').charAt(0).toUpperCase()}</span>
      <span class="txt">
        <b><CodName raw={player.name_raw} /></b>
        <span class="muted">
          {#if player.team}{teamName(player.team)} · {/if}{player.kills ?? 0}/{player.deaths ?? 0} ·
          <span class="dot" style:background={pingColor(player.ping)}></span> {player.ping ?? '—'} ms
        </span>
      </span>
    </div>
    <div class="acts">
      {#if player.key}
        <a class="btn" href={href('joueur', player.key)} onclick={onclose}>{t('Voir la fiche')}</a>
      {/if}
      {#if perms.has('say')}
        <button class="btn" onclick={() => onmessage(player)}>{t('Message privé')}</button>
      {/if}
      {#if perms.has('kick')}
        <button class="btn" disabled={busy} onclick={() => act(() => kick(serverId, player.name_raw))}>{t('Expulser')}</button>
      {/if}
      {#if perms.has('ban')}
        <label class="reason">
          <span>{t('Raison du ban (facultatif)')}</span>
          <input bind:value={reason} maxlength="100" placeholder={t('ex. : triche, insultes…')} />
        </label>
        <div class="bans">
          {#each [['1h', 'Ban 1 h'], ['1d', 'Ban 1 j'], ['7d', 'Ban 7 j']] as [d, label] (d)}
            <button class="btn danger" disabled={busy} onclick={() => act(() => ban(serverId, player.name_raw, d, reason))}>{t(label)}</button>
          {/each}
        </div>
        <form class="custom" onsubmit={e => { e.preventDefault(); if (customOk) act(() => ban(serverId, player.name_raw, custom.trim().toLowerCase(), reason)); }}>
          <label>
            <span class="sr-only">{t('Autre durée')}</span>
            <input bind:value={custom} maxlength="5" placeholder={t('Autre durée : 30m, 12h, 7d')} autocomplete="off" />
          </label>
          <button class="btn danger" disabled={busy || !customOk}>{t('Bannir')}</button>
        </form>
        {#if confirmPerm}
          <button class="btn perm" disabled={busy} onclick={() => act(() => ban(serverId, player.name_raw, '', reason))}>{t('Confirmer le ban définitif')}</button>
        {:else}
          <button class="btn perm outline" onclick={() => (confirmPerm = true)}>{t('Bannir définitivement')}</button>
        {/if}
      {/if}
    </div>
  {/if}
</Sheet>

<style>
  .who { display: flex; align-items: center; gap: 12px; margin-bottom: 14px; }
  .avatar { width: 48px; height: 48px; flex: none; border-radius: 15px; background: var(--surface-2); display: flex; align-items: center; justify-content: center; font-weight: 800; font-size: 18px; }
  .txt { min-width: 0; display: flex; flex-direction: column; }
  .txt b { font-size: 18px; display: flex; min-width: 0; }
  .muted { font-size: 13px; display: flex; align-items: center; gap: 4px; }
  .dot { width: 7px; height: 7px; border-radius: 4px; display: inline-block; }
  .acts { display: flex; flex-direction: column; gap: 8px; }
  .acts .btn { justify-content: flex-start; height: 50px; }
  .reason { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); margin-top: 6px; }
  .reason input { height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .bans { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .bans .btn { justify-content: center; }
  .custom { display: flex; gap: 8px; }
  .custom label { flex: 1; min-width: 0; }
  .custom input { width: 100%; height: 50px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .custom .btn { justify-content: center; }
  .perm { justify-content: center !important; border: 0; background: #E5484D; color: #fff; }
  .perm.outline { background: transparent; border: 1px solid #5A2A30; color: var(--danger-text); }
</style>
