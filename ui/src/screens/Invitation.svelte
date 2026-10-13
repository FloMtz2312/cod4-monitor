<script module>
  // the token of the link, kept here once it is taken out of the address bar
  let held = '';
</script>

<script>
  // Invitation: the link sent by a server owner (valid 48 h, used once) creates the account with the password chosen
  // here, or adds the server to an existing account (its current password).
  import BackTitle from '../components/BackTitle.svelte';
  import { post } from '../lib/api.js';
  import { route, replace, go } from '../lib/router.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  if (route.params[0]) {
    held = route.params[0];
    replace('invitation'); // the token doesn't stay in the history
  }
  const token = held;
  let password = $state('');
  let error = $state('');
  let busy = $state(false);
  let done = $state('');

  async function submit(e) {
    e.preventDefault();
    busy = true;
    error = '';
    try {
      const r = await post('/api/invitations/accept', { token, password });
      done = r.username || '?';
      held = '';
    } catch (err) {
      error = err.status ? t(err.message || 'Invitation impossible') : t('Serveur injoignable');
    } finally {
      busy = false;
    }
  }
</script>

<BackTitle title={t('Invitation')} sub={t('Rejoindre l\'équipe d\'un serveur')} fallback="direct" />

{#if done}
  <div class="card box">
    <h2>{t('Compte prêt')}</h2>
    <p>{t('Ton identifiant : {name}', { name: done })}</p>
    <button class="btn primary" onclick={() => go('connexion')}>{t('Se connecter')}</button>
  </div>
{:else if !token}
  <div class="card box"><p class="muted">{t('Lien d\'invitation incomplet : ouvre le lien reçu en entier, ou demande-en un nouveau.')}</p></div>
{:else}
  <form class="card box" onsubmit={submit}>
    <p class="muted">{t('Choisis ton mot de passe (8 à 128 caractères). Si tu as déjà un compte, utilise ton mot de passe actuel pour rejoindre cette équipe.')}</p>
    <label class="fl"><span>{t('Mot de passe')}</span>
      <input bind:value={password} type="password" autocomplete="new-password" minlength="8" maxlength="128" required />
    </label>
    {#if error}<p class="err" role="alert">{error}</p>{/if}
    <button class="btn primary" disabled={busy || password.length < 8}>{t('Accepter l\'invitation')}</button>
  </form>
{/if}

<style>
  .box { padding: 16px; display: flex; flex-direction: column; gap: 12px; max-width: 420px; }
  .box p, .box h2 { margin: 0; }
  .box h2 { font-size: 17px; }
  .fl { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .fl input { height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .err { color: var(--danger-text); font-size: 14px; }
</style>
