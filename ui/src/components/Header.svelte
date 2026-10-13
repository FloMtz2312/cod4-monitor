<script>
  import Icon from './Icon.svelte';
  import ServerLogo from './ServerLogo.svelte';
  import { app, role, currentServer, mapName } from '../lib/state.svelte.js';
  import { t } from '../lib/i18n.svelte.js';
  import { inboxCount } from '../lib/inbox.svelte.js';

  /** @type {{ onservers: () => void, onsearch: () => void }} */
  let { onservers, onsearch } = $props();

  const srv = $derived(currentServer());
  const r = $derived(role());
  const name = $derived(srv ? srv.name.replace(/\^\d/g, '') : 'CoD4 Monitor');
  const status = $derived(
    !app.loaded ? t('Chargement…')
      : !srv ? t('Aucun serveur surveillé pour le moment.')
      : srv.online ? `${t('En ligne')} · ${srv.players ?? 0}/${srv.max_players ?? '?'} · ${mapName(srv.map)}`
      : t('Hors ligne')
  );
  const todo = $derived(inboxCount(app.serverId));
  const initial = $derived((app.me?.username || 'A').charAt(0).toUpperCase());
</script>

<header>
  <button class="srv" onclick={onservers} aria-label={t('Changer de serveur')} disabled={!app.servers.length}>
    <ServerLogo server={srv} size={36} />
    <span class="txt">
      <span class="name">{name}</span>
      <span class="st"><span class="dot" class:on={srv?.online}></span>{status}</span>
    </span>
    <Icon name="chevronDown" size={18} stroke={2} />
  </button>
  <button class="iconbtn search" onclick={onsearch} aria-label={t('Rechercher')} title={t('Rechercher (Ctrl K)')}><Icon name="search" /></button>
  {#if r === 'visitor'}
    <a class="iconbtn" href="#/connexion" aria-label={t('Se connecter')}><Icon name="user" /></a>
  {:else}
    <a class="iconbtn bell" href="#/moderation" aria-label={todo ? t('Modération : {n} à traiter', { n: todo }) : t('Modération')}><Icon name="bell" />{#if todo}<span class="badge">{todo > 9 ? '9+' : todo}</span>{/if}</a>
    <a class="iconbtn account" href="#/plus" aria-label={t('Mon compte')}>{initial}</a>
  {/if}
</header>

<style>
  header {
    position: sticky; top: 0; z-index: 5; height: calc(var(--header-h) + var(--sat)); display: flex; align-items: center; gap: 8px;
    padding: var(--sat) calc(12px + var(--sar)) 0 calc(12px + var(--sal)); background: var(--bg); border-bottom: 1px solid var(--surface-2);
  }
  .srv {
    flex: 1; min-width: 0; display: flex; align-items: center; gap: 10px; height: 48px; padding: 0 10px 0 6px;
    border-radius: 14px; border: 1px solid var(--line); background: var(--surface); text-align: left; color: var(--muted);
  }
  .txt { display: flex; flex-direction: column; min-width: 0; flex: 1; }
  .name { font-weight: 700; color: var(--text); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .st { font-size: 12px; color: var(--muted); display: flex; align-items: center; gap: 6px; white-space: nowrap; overflow: hidden; }
  .dot { width: 7px; height: 7px; flex: none; border-radius: 4px; background: var(--danger); }
  .dot.on { background: var(--ok); }
  .iconbtn {
    width: 46px; height: 46px; flex: none; border-radius: 14px; border: 1px solid var(--line); background: var(--surface);
    color: var(--text); display: flex; align-items: center; justify-content: center; text-decoration: none;
  }
  .bell { position: relative; }
  .badge { position: absolute; top: 6px; right: 6px; min-width: 18px; height: 18px; padding: 0 5px; border-radius: 9px; background: var(--danger); color: #fff; font-size: 11px; font-weight: 800; display: flex; align-items: center; justify-content: center; }
  .account { background: var(--accent); color: var(--on-accent); border: 0; font-weight: 800; }
  .search { display: none; }
  @media (min-width: 960px) {
    header { padding: var(--sat) 28px 0; }
    .srv { flex: 0 1 380px; }
    .search { display: flex; margin-left: auto; }
  }
</style>
