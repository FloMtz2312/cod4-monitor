<script>
  // Comptes (super admin): the dashboard accounts, a new password for one (signs it out everywhere), or delete it.
  import BackTitle from '../components/BackTitle.svelte';
  import Sheet from '../components/Sheet.svelte';
  import { api, get, post } from '../lib/api.js';
  import { app } from '../lib/state.svelte.js';
  import { dt } from '../lib/moderation.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const ROLE = { admin: 'Administrateur', moderator: 'Modérateur' };
  const allowed = $derived(!!app.me?.admin);
  let users = $state(null);
  let admin = $state('');
  let pwFor = $state('');
  let pw = $state('');
  let confirmDel = $state('');
  let busy = $state(false);

  const load = () => get('/api/users').then(r => { users = r.users ?? []; admin = r.admin ?? ''; }).catch(e => { users = []; say(t(e.message)); });
  $effect(() => { if (allowed) load(); });

  async function setPw(e) {
    e.preventDefault();
    if (pw.length < 8) { say(t('le mot de passe doit faire au moins 8 caractères')); return; }
    busy = true;
    try {
      await post(`/api/users/${encodeURIComponent(pwFor)}/password`, { password: pw });
      say(t('Mot de passe changé (le compte est déconnecté partout)'));
      pwFor = '';
    } catch (err) {
      say(t('Échec : {err}', { err: t(err.message) }));
    } finally {
      busy = false;
    }
  }

  async function del(name) {
    busy = true;
    try {
      await api('DELETE', `/api/users/${encodeURIComponent(name)}`);
      say(t('Compte supprimé'));
      load();
    } catch (err) {
      say(t('Échec : {err}', { err: t(err.message) }));
    } finally {
      busy = false;
      confirmDel = '';
    }
  }
</script>

<BackTitle title={t('Comptes du dashboard')} sub={t('Les administrateurs rejoignent sur invitation. Chaque serveur a sa propre équipe.')} />

{#if !allowed}
  <p class="muted">{t('Réservé au super admin.')}</p>
{:else if users === null}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <div class="card rows">
    <div class="row">
      <span class="who"><b>{admin}</b><small class="muted">{t('Super admin')} · .env</small></span>
    </div>
    {#each users as u (u.username)}
      <div class="row">
        <span class="who">
          <b>{u.username} <span class="badge">{t(ROLE[u.account_role] ?? u.account_role)}</span></b>
          <small class="muted">{t('créé le {d}', { d: dt(u.created_at) })} · {u.last_login ? t('dernière connexion {d}', { d: dt(u.last_login) }) : t('jamais connecté')}</small>
        </span>
        <span class="acts">
          <button class="btn" onclick={() => { pwFor = u.username; pw = ''; }}>{t('Nouveau mot de passe')}</button>
          {#if confirmDel === u.username}
            <button class="btn danger" disabled={busy} onclick={() => del(u.username)}>{t('Confirmer')}</button>
            <button class="btn ghost" onclick={() => (confirmDel = '')}>{t('Annuler')}</button>
          {:else}
            <button class="btn ghost danger-t" onclick={() => (confirmDel = u.username)}>{t('Supprimer')}</button>
          {/if}
        </span>
      </div>
    {:else}
      <p class="muted empty">{t('Aucun autre compte. Invite des administrateurs depuis Équipe et invitations.')}</p>
    {/each}
  </div>
{/if}

<Sheet open={!!pwFor} onclose={() => (pwFor = '')} label={t('Nouveau mot de passe')}>
  <form class="pw" onsubmit={setPw}>
    <h2>{t('Nouveau mot de passe pour {name}', { name: pwFor })}</h2>
    <p class="muted">{t('8 caractères minimum. Le compte est déconnecté de tous ses appareils.')}</p>
    <input type="password" bind:value={pw} autocomplete="new-password" minlength="8" maxlength="128" aria-label={t('Nouveau mot de passe')} />
    <button class="btn primary" disabled={busy || pw.length < 8}>{t('Enregistrer')}</button>
  </form>
</Sheet>

<style>
  .rows { overflow: hidden; }
  .row { display: flex; flex-wrap: wrap; align-items: center; justify-content: space-between; gap: 10px; padding: 12px 16px; border-top: 1px solid var(--line-soft); }
  .row:first-child { border-top: 0; }
  .who { display: flex; flex-direction: column; gap: 2px; min-width: 0; flex: 1 1 260px; }
  .who small { font-size: 12px; }
  .badge { font-size: 11px; font-weight: 800; padding: 2px 7px; border-radius: 7px; background: var(--surface-2); color: var(--text-2); margin-left: 4px; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; }
  .acts .btn { min-height: 40px; padding: 0 12px; font-size: 14px; }
  .danger-t { color: var(--danger-text); }
  .empty { margin: 0; padding: 14px 16px; border-top: 1px solid var(--line-soft); }
  .pw { display: flex; flex-direction: column; gap: 10px; }
  .pw h2 { font-size: 17px; }
  .pw p { margin: 0; font-size: 13px; }
  .pw input { height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
</style>
