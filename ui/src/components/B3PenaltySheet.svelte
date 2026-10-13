<script>
  // B3 sanction from the profile (also offline): warning, temp ban or ban, with a reason. B3 enforces it and keeps
  // its history; a player online is also warned / kicked at once.
  import Sheet from './Sheet.svelte';
  import CodName from './CodName.svelte';
  import { post } from '../lib/api.js';
  import { safeText } from '../lib/actions.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void, serverId: number, guid: string, nameRaw: string, ondone: () => void, online?: boolean }} */
  let { open, onclose, serverId, guid, nameRaw, ondone, online = false } = $props();

  const KINDS = {
    Warning: { label: 'Avertir', durs: [[60, '1 h'], [1440, '1 j'], [10080, '7 j'], [43200, '30 j']],
      hint: 'Message privé au joueur s\'il est en jeu. B3 compte les avertissements en cours (trop d\'avertissements = kick puis ban temporaire).' },
    Kick: { label: 'Kick', durs: [], hint: 'Le joueur est expulsé tout de suite ; il peut revenir.' },
    TempBan: { label: 'Ban temporaire', durs: [[60, '1 h'], [360, '6 h'], [1440, '1 j'], [4320, '3 j'], [10080, '7 j'], [43200, '30 j']],
      hint: 'Expulsé tout de suite ; B3 le refuse à chaque connexion jusqu\'à la fin du ban.' },
    Ban: { label: 'Ban définitif', durs: [], hint: 'Définitif : B3 le refuse à chaque connexion, jusqu\'à ce que le ban soit levé.' }
  };
  const REASONS = ['Triche', 'Insultes', 'Spam', 'Antijeu', 'Camping', 'Pseudo inapproprié'];
  let kind = $state('Warning');
  let minutes = $state(1440);
  let reason = $state('');
  let busy = $state(false);

  $effect(() => { if (open) { kind = 'Warning'; minutes = 1440; reason = ''; } });
  $effect(() => {
    const d = KINDS[kind].durs;
    if (d.length && !d.some(([m]) => m === minutes)) minutes = 1440;
  });
  $effect(() => { if (kind === 'Kick' && !online) kind = 'Warning'; });

  async function submit(e) {
    e.preventDefault();
    const r = safeText(reason);
    if (!r) { say(t('Indique une raison')); return; }
    busy = true;
    try {
      const res = await post(`/api/servers/${serverId}/b3/penalty`, { guid, name: nameRaw, type: kind, minutes: KINDS[kind].durs.length ? minutes : 0, reason: r });
      say(t('Sanction B3 enregistrée') + (res.online || kind === 'Warning' ? '' : ` (${t('hors ligne : appliqué à sa prochaine connexion')})`));
      ondone();
      onclose();
    } catch (err) {
      say(t('Échec : {err}', { err: err.message }));
    } finally {
      busy = false;
    }
  }
</script>

<Sheet {open} {onclose} label={t('Sanction B3')}>
  <form onsubmit={submit}>
    <h2 class="hd">{t('Sanction B3')} · <CodName raw={nameRaw} /></h2>
    <div class="kinds" role="radiogroup" aria-label={t('Type de sanction')}>
      {#each Object.entries(KINDS) as [k, v] (k)}
        <button type="button" role="radio" aria-checked={kind === k} class:on={kind === k} class:danger={k === 'Ban'} disabled={k === 'Kick' && !online}
          title={k === 'Kick' && !online ? t('Le joueur n\'est pas en jeu') : null} onclick={() => (kind = k)}>{t(v.label)}</button>
      {/each}
    </div>
    {#if KINDS[kind].durs.length}
      <div class="durs" role="radiogroup" aria-label={t('Durée')}>
        {#each KINDS[kind].durs as [m, label] (m)}
          <button type="button" role="radio" aria-checked={minutes === m} class:on={minutes === m} onclick={() => (minutes = m)}>{t(label)}</button>
        {/each}
      </div>
    {/if}
    <label class="reason">
      <span>{t('Raison (obligatoire)')}</span>
      <input bind:value={reason} maxlength="120" placeholder={t('Visible par le joueur')} required />
    </label>
    <div class="chips">
      {#each REASONS as r (r)}<button type="button" class:on={reason === t(r)} onclick={() => (reason = t(r))}>{t(r)}</button>{/each}
    </div>
    <p class="hint">{t(KINDS[kind].hint)}</p>
    <button class="btn submit" class:primary={kind !== 'Ban'} class:perm={kind === 'Ban'} disabled={busy}>
      {kind === 'Ban' ? t('Confirmer le ban définitif') : kind === 'Kick' ? t('Expulser') : kind === 'TempBan' ? t('Bannir temporairement') : t('Avertir')}
    </button>
  </form>
</Sheet>

<style>
  form { display: flex; flex-direction: column; gap: 12px; }
  .hd { font-size: 17px; display: flex; flex-wrap: wrap; gap: 6px; }
  .kinds, .durs { display: flex; gap: 6px; flex-wrap: wrap; }
  .kinds button, .durs button { height: 40px; padding: 0 14px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); font-weight: 700; }
  .kinds button.on, .durs button.on { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .kinds button.danger.on { border-color: #5A2A30; background: rgba(255, 99, 105, .12); color: var(--danger-text); }
  .reason { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .reason input { height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .kinds button:disabled { opacity: .45; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chips button { height: 32px; padding: 0 10px; border-radius: 10px; border: 1px solid var(--line); background: transparent; color: var(--text-2); font-size: 13px; font-weight: 700; }
  .chips button.on { border-color: var(--accent); color: var(--accent); }
  .hint { margin: 0; font-size: 12px; color: var(--muted); }
  .submit { height: 50px; }
  .perm { border: 0; background: #E5484D; color: #fff; }
</style>
