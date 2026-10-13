<script>
  // Connexion: the staff signs in (accounts are created by invitation); then back to the screen it came from.
  import BackTitle from '../components/BackTitle.svelte';
  import { post } from '../lib/api.js';
  import { app, role, loadMe, loadServers } from '../lib/state.svelte.js';
  import { back, go } from '../lib/router.svelte.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let busy = $state(false);
  const signedIn = $derived(app.me && role() !== 'visitor');

  async function submit(e) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      await post('/api/login', { username, password });
      await loadMe();
      loadServers();
      say(t('Connecté'));
      back('direct');
    } catch (err) {
      error = err.status ? t(err.message || 'Connexion impossible') : t('Serveur injoignable');
      password = '';
    } finally {
      busy = false;
    }
  }
</script>

<BackTitle title={t('Connexion')} sub={t('Pour l\'équipe des serveurs : les comptes se créent sur invitation.')} fallback="plus" />

{#if signedIn}
  <div class="card box">
    <p>{t('Tu es connecté en tant que {name}.', { name: app.me.username || t('Admin') })}</p>
    <button class="btn primary" onclick={() => go('direct')}>{t('Continuer')}</button>
  </div>
{:else}
  <form class="card box" onsubmit={submit}>
    <label class="fl"><span>{t('Identifiant')}</span>
      <input bind:value={username} autocomplete="username" autocapitalize="none" spellcheck="false" required maxlength="128" />
    </label>
    <label class="fl"><span>{t('Mot de passe')}</span>
      <input bind:value={password} type="password" autocomplete="current-password" required maxlength="128" />
    </label>
    {#if error}<p class="err" role="alert">{error}</p>{/if}
    <button class="btn primary" disabled={busy || !username.trim() || !password}>{t('Se connecter')}</button>
    <button type="button" class="btn ghost" onclick={() => go('direct')}>{t('Continuer sans compte (stats publiques)')}</button>
  </form>
{/if}

<style>
  .box { padding: 16px; display: flex; flex-direction: column; gap: 12px; max-width: 420px; }
  .box p { margin: 0; }
  .fl { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .fl input { height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .err { color: var(--danger-text); font-size: 14px; }
</style>
