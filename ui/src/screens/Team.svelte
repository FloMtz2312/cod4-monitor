<script>
  // Équipe et invitations of the selected server (owner): the members and their permissions, the invitations
  // (a link valid 48 h that creates the account), and for the super admin, inviting another server administrator.
  import BackTitle from '../components/BackTitle.svelte';
  import { api, get, post } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { isOwner } from '../lib/perms.js';
  import { agoText } from '../lib/format.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const ORDER = ['private', 'reports', 'kick', 'ban', 'notes', 'say', 'maps', 'rcon'];
  const srv = $derived(currentServer());
  const id = $derived(app.serverId);
  const owner = $derived(isOwner(app.me, id));
  const superAdmin = $derived(!!(app.me?.admin || app.me?.noauth));

  let members = $state(null);
  let labels = $state({});
  let edits = $state({}); // username -> permissions being edited
  let invites = $state([]);
  let confirmRemove = $state('');
  let form = $state({ username: '', role: 'moderator', perms: ['private', 'reports', 'kick'] });
  let link = $state('');
  let busy = $state(false);

  const parse = p => { try { return Array.isArray(p) ? p : JSON.parse(p || '[]'); } catch { return []; } };

  async function load(sid) {
    try {
      const r = await get(`/api/servers/${sid}/team`);
      members = r.members ?? [];
      labels = r.permissions ?? {};
      edits = Object.fromEntries(members.map(m => [m.username, parse(m.permissions)]));
    } catch {
      members = [];
    }
    try {
      const r = await get('/api/invitations');
      invites = (r.invitations ?? []).filter(i => i.server_id === sid || (i.account_role === 'admin' && superAdmin));
      if (!Object.keys(labels).length) labels = r.permissions ?? {};
    } catch {
      invites = [];
    }
  }

  $effect(() => { if (id && owner) load(id); });

  const toggle = (list, p) => (list.includes(p) ? list.filter(x => x !== p) : [...list, p]);
  const same = (a, b) => a.length === b.length && a.every(x => b.includes(x));

  async function saveMember(m) {
    try {
      await api('PUT', `/api/servers/${id}/team/${encodeURIComponent(m.username)}`, { permissions: edits[m.username] });
      m.permissions = JSON.stringify(edits[m.username]);
      say(t('Permissions de {name} enregistrées', { name: m.username }));
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    }
  }

  async function remove(name) {
    try {
      await api('DELETE', `/api/servers/${id}/team/${encodeURIComponent(name)}`);
      say(t('{name} retiré de l\'équipe', { name }));
      confirmRemove = '';
      load(id);
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    }
  }

  async function invite(e) {
    e.preventDefault();
    busy = true;
    link = '';
    try {
      const body = form.role === 'admin' ? { username: form.username, role: 'admin' }
        : { username: form.username, role: 'moderator', server_id: id, permissions: form.perms };
      const r = await post('/api/invitations', body);
      link = location.origin + r.path;
      form.username = '';
      load(id);
    } catch (err) {
      say(t('Échec : {err}', { err: err.message }));
    } finally {
      busy = false;
    }
  }

  async function share() {
    try {
      if (navigator.share) await navigator.share({ title: t('Invitation'), url: link });
      else { await navigator.clipboard.writeText(link); say(t('Lien copié')); }
    } catch {}
  }

  async function revoke(inv) {
    try {
      await api('DELETE', `/api/invitations/${inv.id}`);
      say(t('Invitation révoquée'));
      load(id);
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    }
  }

  const now = Date.now() / 1000;
  const inviteStatus = i => i.used_at ? t('acceptée') : i.revoked_at ? t('révoquée') : i.expires_at < now ? t('expirée') : t('en attente');
  const pending = i => !i.used_at && !i.revoked_at && i.expires_at >= now;
</script>

<BackTitle title={t('Équipe et invitations')} sub={srv ? srv.name.replace(/\^\d/g, '') : ''} />

{#if !owner}
  <p class="muted">{t('Réservé au propriétaire du serveur.')}</p>
{:else}
  <section aria-label={t('Équipe')}>
    <h2 class="label sec">{t('Équipe')}</h2>
    {#if members === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else if !members.length}
      <div class="card box"><p class="muted">{t('Personne pour l\'instant : invite un modérateur ci-dessous.')}</p></div>
    {:else}
      <div class="list">
        {#each members as m (m.username)}
          <div class="card box">
            <div class="mh"><span class="av">{m.username.charAt(0).toUpperCase()}</span><b>{m.username}</b></div>
            <div class="perms">
              {#each ORDER as p (p)}
                <button type="button" aria-pressed={edits[m.username]?.includes(p)} onclick={() => (edits[m.username] = toggle(edits[m.username] ?? [], p))}>{t(labels[p] ?? p)}</button>
              {/each}
            </div>
            <div class="acts">
              {#if !same(edits[m.username] ?? [], parse(m.permissions))}<button class="btn primary" onclick={() => saveMember(m)}>{t('Enregistrer')}</button>{/if}
              {#if confirmRemove === m.username}
                <button class="btn danger" onclick={() => remove(m.username)}>{t('Confirmer le retrait')}</button>
              {:else}
                <button class="btn ghost" onclick={() => (confirmRemove = m.username)}>{t('Retirer de l\'équipe')}</button>
              {/if}
            </div>
          </div>
        {/each}
      </div>
    {/if}
  </section>

  <section aria-label={t('Inviter')}>
    <h2 class="label sec">{t('Inviter')}</h2>
    <form class="card box" onsubmit={invite}>
      <label class="fl"><span>{t('Identifiant du compte')}</span>
        <input bind:value={form.username} required minlength="3" maxlength="32" autocomplete="off" autocapitalize="none" spellcheck="false" placeholder={t('ex. : werner')} />
      </label>
      {#if superAdmin}
        <div class="roles" role="radiogroup" aria-label={t('Rôle')}>
          <button type="button" role="radio" aria-checked={form.role === 'moderator'} onclick={() => (form.role = 'moderator')}>{t('Modérateur de ce serveur')}</button>
          <button type="button" role="radio" aria-checked={form.role === 'admin'} onclick={() => (form.role = 'admin')}>{t('Administrateur (ses propres serveurs)')}</button>
        </div>
      {/if}
      {#if form.role === 'moderator'}
        <div class="perms">
          {#each ORDER as p (p)}
            <button type="button" aria-pressed={form.perms.includes(p)} onclick={() => (form.perms = toggle(form.perms, p))}>{t(labels[p] ?? p)}</button>
          {/each}
        </div>
      {/if}
      <button class="btn primary" disabled={busy || form.username.trim().length < 3}>{t('Créer le lien d\'invitation')}</button>
      {#if link}
        <div class="link">
          <p>{t('Envoie ce lien à la personne : il est valable 48 h et ne sert qu\'une fois. Elle y choisira son mot de passe.')}</p>
          <code>{link}</code>
          <button type="button" class="btn" onclick={share}>{t('Partager ou copier le lien')}</button>
        </div>
      {/if}
    </form>
  </section>

  {#if invites.length}
    <section aria-label={t('Invitations')}>
      <h2 class="label sec">{t('Invitations')}</h2>
      <div class="card rows">
        {#each invites as i (i.id)}
          <div class="row">
            <span class="rt"><b>{i.username}</b><span class="muted">{i.account_role === 'admin' ? t('administrateur') : t('modérateur')} · {inviteStatus(i)} · {agoText(i.created_at)}</span></span>
            {#if pending(i)}<button class="btn ghost" onclick={() => revoke(i)}>{t('Révoquer')}</button>{/if}
          </div>
        {/each}
      </div>
    </section>
  {/if}
{/if}

<style>
  section { margin-top: 6px; }
  .sec { display: block; margin: 12px 2px 8px; }
  .list { display: grid; gap: 10px; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; }
  .box p { margin: 0; }
  .mh { display: flex; align-items: center; gap: 10px; }
  .av { width: 36px; height: 36px; border-radius: 11px; background: var(--surface-2); display: flex; align-items: center; justify-content: center; font-weight: 800; }
  .perms, .roles { display: flex; flex-wrap: wrap; gap: 6px; }
  .perms button, .roles button { min-height: 34px; padding: 6px 10px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); font-weight: 700; font-size: 12px; text-align: left; }
  .perms button[aria-pressed='true'], .roles button[aria-checked='true'] { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; }
  .acts .btn { min-height: 40px; }
  .fl { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .fl input { height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; }
  .link { display: flex; flex-direction: column; gap: 8px; padding: 12px; border-radius: 12px; background: var(--accent-soft); }
  .link p { font-size: 13px; }
  .link code { font-size: 12px; overflow-wrap: anywhere; color: var(--text); }
  .rows { overflow: hidden; }
  .row { display: flex; align-items: center; gap: 10px; padding: 10px 14px; border-top: 1px solid var(--line-soft); }
  .row:first-child { border-top: 0; }
  .rt { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .rt .muted { font-size: 12px; }
  .row .btn { min-height: 36px; }
</style>
