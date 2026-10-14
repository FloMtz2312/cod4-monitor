<script>
  // Plus: the account, then everything that is not a tab — statistics, servers, my permissions, the administration
  // of the selected server (according to the rights), and the preferences.
  import Icon from '../components/Icon.svelte';
  import ServerLogo from '../components/ServerLogo.svelte';
  import { app, role, loadMe, currentServer } from '../lib/state.svelte.js';
  import { roleLabel } from '../lib/roles.js';
  import { permsOf, isOwner } from '../lib/perms.js';
  import { href } from '../lib/router.svelte.js';
  import { i18n, setLang, t, LANGS } from '../lib/i18n.svelte.js';
  import { REPO, NEW_ISSUE } from '../lib/project.js';
  import { post } from '../lib/api.js';
  import { say } from '../lib/toast.svelte.js';

  const PERMS = {
    private: 'Consulter les données privées (IP, chat, logs, notes)', reports: 'Traiter les signalements', kick: 'Expulser un joueur',
    ban: 'Bannir et débannir', notes: 'Modifier les notes joueurs', say: 'Envoyer un message', maps: 'Changer la map / relancer la partie',
    rcon: 'Console RCON complète et commandes programmées (contrôle total du jeu)'
  };

  const r = $derived(role());
  const srv = $derived(currentServer());
  const id = $derived(app.serverId);
  const name = $derived(app.me?.username || '');
  const perms = $derived(permsOf(app.me, id));
  const owner = $derived(isOwner(app.me, id));
  const accountAdmin = $derived(['admin', 'superadmin'].includes(app.me?.role) || !!app.me?.noauth);
  const superAdmin = $derived(!!app.me?.admin);
  const label = $derived(superAdmin ? t('Super admin') : app.me?.role === 'admin' ? t('Administrateur') : t(roleLabel[r]));
  const srvName = $derived(srv ? srv.name.replace(/\^\d/g, '') : '');
  let showPerms = $state(false);

  async function logout() {
    try { await post('/api/logout'); } catch {}
    await loadMe();
    say(t('Déconnecté'));
  }

  function install() {
    say(/iphone|ipad/i.test(navigator.userAgent)
      ? t('Dans Safari : bouton Partager, puis « Sur l\'écran d\'accueil ».')
      : t('Dans le menu du navigateur : « Installer l\'application » ou « Ajouter à l\'écran d\'accueil ».'));
  }
</script>

<h1 class="title">{t('Plus')}</h1>

{#if r === 'visitor'}
  <section class="card account">
    <h2>{t('Tu administres ou modères un serveur ?')}</h2>
    <p>{t('Connecte-toi pour voir le chat, les signalements et agir sur la partie. Les comptes se créent sur invitation.')}</p>
    <a class="btn primary" href={href('connexion')}>{t('Se connecter')}</a>
  </section>
{:else}
  <section class="card me">
    <span class="avatar">{(name || 'A').charAt(0).toUpperCase()}</span>
    <span><b>{name || t('Admin')}</b><span class="muted">{app.me?.noauth ? t('Accès sans compte (pas de mot de passe défini)') : label}</span></span>
  </section>
{/if}

<section>
  <h2 class="label group">{t('Statistiques')}</h2>
  <div class="card rows">
    <a class="row" href={href('stats')}><Icon name="chart" size={20} /><span>{t('Stats avancées')}<small>{t('affluence, maps, votes, fidélité, armes')}</small></span><Icon name="chevronRight" size={18} /></a>
    <a class="row" href={href('matchs')}><Icon name="matches" size={20} /><span>{t('Matchs récents')}<small>{t('tableaux des scores, rounds, MVP')}</small></span><Icon name="chevronRight" size={18} /></a>
    <a class="row" href={href('etat')}><Icon name="pulse" size={20} /><span>{t('État du serveur')}<small>{t('disponibilité, coupures et leur cause')}</small></span><Icon name="chevronRight" size={18} /></a>
  </div>
</section>

<section>
  <h2 class="label group">{t('Serveurs')}</h2>
  <div class="card rows">
    <a class="row" href={href('serveurs')}><Icon name="globe" size={20} /><span>{t('Tous les serveurs')}</span><span class="val">{app.servers.length}</span><Icon name="chevronRight" size={18} /></a>
  </div>
</section>

{#if r !== 'visitor' && srv && !owner}
  <section>
    <h2 class="label group">{t('Mes droits sur {name}', { name: srvName })}</h2>
    <div class="card rows">
      <button class="row" onclick={() => (showPerms = !showPerms)} aria-expanded={showPerms}>
        <Icon name="key" size={20} /><span>{t('Mes permissions')}</span><span class="val">{perms.size}</span><Icon name={showPerms ? 'chevronDown' : 'chevronRight'} size={18} />
      </button>
      {#if showPerms}
        <ul class="perms">
          {#each Object.entries(PERMS) as [k, text] (k)}<li class:off={!perms.has(k)}><Icon name={perms.has(k) ? 'check' : 'more'} size={16} />{t(text)}</li>{/each}
        </ul>
      {/if}
    </div>
  </section>
{/if}

{#if r !== 'visitor' && srv && (owner || perms.has('rcon') || perms.has('private') || accountAdmin)}
  <section>
    <h2 class="label group">{t('Administration de {name}', { name: srvName })}</h2>
    <div class="card rows">
      {#if owner}
        <a class="row" href={href('personnalisation')}><ServerLogo server={srv} size={22} /><span>{t('Personnalisation')}<small>{t('couleur et logo du serveur')}</small></span><Icon name="chevronRight" size={18} /></a>
        <a class="row" href={href('equipe')}><Icon name="users" size={20} /><span>{t('Équipe et invitations')}<small>{t('modérateurs et leurs permissions')}</small></span><Icon name="chevronRight" size={18} /></a>
        {#if accountAdmin}<a class="row" href={href('reglages', String(srv.id))}><Icon name="settings" size={20} /><span>{t('Réglages du serveur')}<small>{t('adresse, rcon, logs')}</small></span><Icon name="chevronRight" size={18} /></a>{/if}
      {/if}
      {#if perms.has('rcon') || perms.has('private')}
        <a class="row" href={href('console')}><Icon name="terminal" size={20} /><span>{t('Console')}<small>{perms.has('rcon') ? t('console du jeu en direct, commandes RCON') : t('console du jeu en direct')}</small></span><Icon name="chevronRight" size={18} /></a>
      {/if}
      {#if perms.has('rcon')}
        <a class="row" href={href('programmes')}><Icon name="clock" size={20} /><span>{t('Commandes programmées')}<small>{t('messages, redémarrages réguliers')}</small></span><Icon name="chevronRight" size={18} /></a>
      {/if}
      {#if superAdmin}
        <a class="row" href={href('comptes')}><Icon name="users" size={20} /><span>{t('Comptes du dashboard')}<small>{t('mots de passe, suppression')}</small></span><Icon name="chevronRight" size={18} /></a>
      {/if}
      {#if accountAdmin}
        <a class="row" href={href('journal')}><Icon name="list" size={20} /><span>{t('Journal d\'activité')}<small>{t('qui a fait quoi depuis le site')}</small></span><Icon name="chevronRight" size={18} /></a>
      {/if}
    </div>
  </section>
{/if}

<section>
  <h2 class="label group">{t('Préférences')}</h2>
  <div class="card rows">
    <button class="row" onclick={() => setLang(LANGS[(LANGS.findIndex(([k]) => k === i18n.lang) + 1) % LANGS.length][0])}>
      <Icon name="globe" size={20} /><span>{t('Langue')}</span><span class="val">{LANGS.find(([k]) => k === i18n.lang)?.[1]}</span><Icon name="chevronRight" size={18} />
    </button>
    {#if r !== 'visitor'}
      <a class="row" href={href('notifications')}><Icon name="bell" size={20} /><span>{t('Notifications')}<small>{t('serveur tombé, signalements, suspects, bans')}</small></span><Icon name="chevronRight" size={18} /></a>
    {/if}
    <button class="row" onclick={install}><Icon name="phone" size={20} /><span>{t('Installer l\'application')}<small>{t('sur l\'écran d\'accueil du téléphone')}</small></span><Icon name="chevronRight" size={18} /></button>
    {#if r !== 'visitor' && !app.me?.noauth && !superAdmin}
      <a class="row" href={href('motdepasse')}><Icon name="key" size={20} /><span>{t('Mot de passe')}</span><Icon name="chevronRight" size={18} /></a>
    {/if}
    {#if r !== 'visitor' && !app.me?.noauth}
      <button class="row danger" onclick={logout}><Icon name="logout" size={20} /><span>{t('Se déconnecter')}</span></button>
    {/if}
  </div>
</section>

<section>
  <h2 class="label group">{t('À propos')}</h2>
  <div class="card rows">
    <a class="row" href={NEW_ISSUE} target="_blank" rel="noopener"><Icon name="alert" size={20} /><span>{t('Signaler un bug ou proposer une idée')}<small>{t('sur GitHub, en français ou en anglais')}</small></span><Icon name="external" size={18} /></a>
    <a class="row" href={REPO} target="_blank" rel="noopener"><Icon name="link" size={20} /><span>{t('Code source')}<small>{t('CoD4 Monitor, logiciel libre (AGPL-3.0)')}</small></span><Icon name="external" size={18} /></a>
  </div>
</section>

<style>
  section { margin-top: 14px; }
  .account { padding: 16px; display: flex; flex-direction: column; gap: 10px; }
  .account h2 { font-size: 16px; }
  .account p { margin: 0; font-size: 13px; color: var(--text-2); }
  .me { padding: 14px 16px; display: flex; align-items: center; gap: 12px; }
  .me b { display: block; }
  .me .muted { display: block; font-size: 13px; }
  .avatar { width: 44px; height: 44px; border-radius: 14px; background: var(--accent); color: var(--on-accent); display: flex; align-items: center; justify-content: center; font-weight: 800; font-size: 18px; }
  .group { display: block; margin: 0 0 8px 4px; font-size: 12px; color: var(--muted); }
  .rows { overflow: hidden; }
  .row {
    width: 100%; display: flex; align-items: center; gap: 12px; min-height: 54px; padding: 8px 16px; border: 0;
    border-top: 1px solid var(--line-soft); background: transparent; color: var(--text); text-align: left;
    font-size: 15px; font-weight: 700; text-decoration: none;
  }
  .row:first-child { border-top: 0; }
  .row > span:not(.val) { flex: 1; min-width: 0; }
  .row small { display: block; font-size: 12px; color: var(--muted); font-weight: 500; }
  .val { font-size: 13px; color: var(--muted); font-weight: 600; }
  .row :global(svg) { color: var(--muted); flex: none; }
  .danger, .danger :global(svg) { color: var(--danger-text) !important; }
  .perms { list-style: none; margin: 0; padding: 4px 16px 12px 48px; display: flex; flex-direction: column; gap: 6px; font-size: 13px; }
  .perms li { display: flex; gap: 8px; align-items: flex-start; color: var(--text); }
  .perms li.off { color: var(--muted); }
  .perms li :global(svg) { margin-top: 1px; }
</style>
