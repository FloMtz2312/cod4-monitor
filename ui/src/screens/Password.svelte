<script>
  // Changer mon mot de passe (accounts created by invitation; the main account's is in the .env).
  import BackTitle from '../components/BackTitle.svelte';
  import { post } from '../lib/api.js';
  import { back } from '../lib/router.svelte.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  let current = $state('');
  let next = $state('');
  let again = $state('');
  let busy = $state(false);
  let error = $state('');

  async function save(e) {
    e.preventDefault();
    error = '';
    if (next !== again) { error = t('Les deux nouveaux mots de passe ne sont pas identiques.'); return; }
    busy = true;
    try {
      await post('/api/me/password', { current, new: next });
      say(t('Mot de passe changé'));
      back('plus');
    } catch (err) {
      error = err.message;
    } finally {
      busy = false;
    }
  }
</script>

<BackTitle title={t('Mot de passe')} />

<form class="card box" onsubmit={save}>
  <label class="fl"><span>{t('Mot de passe actuel')}</span><input type="password" bind:value={current} autocomplete="current-password" required /></label>
  <label class="fl"><span>{t('Nouveau mot de passe')}</span><input type="password" bind:value={next} autocomplete="new-password" minlength="8" required /></label>
  <label class="fl"><span>{t('Encore une fois')}</span><input type="password" bind:value={again} autocomplete="new-password" minlength="8" required /></label>
  <p class="muted small">{t('8 caractères minimum.')}</p>
  {#if error}<p class="err">{error}</p>{/if}
  <button class="btn primary" disabled={busy}>{t('Changer le mot de passe')}</button>
</form>

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; }
  .fl { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .fl input { height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; color: var(--text); }
  p { margin: 0; }
  .small { font-size: 12px; }
  .err { color: var(--danger-text); font-size: 14px; }
</style>
