<script>
  import { onMount } from 'svelte';
  import Header from './components/Header.svelte';
  import BottomNav from './components/BottomNav.svelte';
  import ServerSheet from './components/ServerSheet.svelte';
  import Toast from './components/Toast.svelte';
  import SearchPalette from './components/SearchPalette.svelte';
  import Direct from './screens/Direct.svelte';
  import Servers from './screens/Servers.svelte';
  import More from './screens/More.svelte';
  import Moderation from './screens/Moderation.svelte';
  import Brand from './screens/Brand.svelte';
  import Team from './screens/Team.svelte';
  import Console from './screens/Console.svelte';
  import Schedules from './screens/Schedules.svelte';
  import Journal from './screens/Journal.svelte';
  import Password from './screens/Password.svelte';
  import Status from './screens/Status.svelte';
  import ServerSettings from './screens/ServerSettings.svelte';
  import Players from './screens/Players.svelte';
  import Player from './screens/Player.svelte';
  import Claim from './screens/Claim.svelte';
  import Rankings from './screens/Rankings.svelte';
  import Matches from './screens/Matches.svelte';
  import Match from './screens/Match.svelte';
  import Duel from './screens/Duel.svelte';
  import Stats from './screens/Stats.svelte';
  import Notifications from './screens/Notifications.svelte';
  import Login from './screens/Login.svelte';
  import Invitation from './screens/Invitation.svelte';
  import Accounts from './screens/Accounts.svelte';
  import Bans from './screens/Bans.svelte';
  import Chat from './screens/Chat.svelte';
  import B3 from './screens/B3.svelte';
  import Evasions from './screens/Evasions.svelte';
  import Linked from './screens/Linked.svelte';
  import Demos from './screens/Demos.svelte';
  import { app, role, loadMe, loadServers, selectServer } from './lib/state.svelte.js';
  import { tabsFor } from './lib/roles.js';
  import { permsOf } from './lib/perms.js';
  import { loadInbox } from './lib/inbox.svelte.js';
  import { applyAccent } from './lib/brand.js';
  import { currentServer } from './lib/state.svelte.js';
  import { route, go } from './lib/router.svelte.js';
  import { sync as syncPush } from './lib/push.js';
  import { i18n, t } from './lib/i18n.svelte.js';

  let serversOpen = $state(false);
  let searchOpen = $state(false);

  // Ctrl K (⌘K) anywhere, or / outside a text field: the search
  function shortcut(e) {
    const typing = e.target instanceof HTMLElement && e.target.closest('input, textarea, select, [contenteditable]');
    if ((e.key === 'k' || e.key === 'K') && (e.ctrlKey || e.metaKey)) { e.preventDefault(); searchOpen = true; }
    else if (e.key === '/' && !typing && !e.ctrlKey && !e.metaKey && !e.altKey) { e.preventDefault(); searchOpen = true; }
  }
  const tabs = $derived(tabsFor(role()));

  // a tapped notification opens /?s=<server>#/<screen>: select that server, then forget the parameter
  const opened = Number(new URLSearchParams(location.search).get('s'));
  if (opened) {
    selectServer(opened);
    history.replaceState(null, '', location.pathname + location.hash);
  }

  onMount(() => {
    document.documentElement.lang = i18n.lang;
    loadMe().then(() => { if (app.me && app.me.role !== 'visitor') syncPush().catch(() => {}); });
    loadServers();
    const every = setInterval(loadServers, 20000);
    const back = () => { if (!document.hidden) { loadMe(); loadServers(); } };
    document.addEventListener('visibilitychange', back);
    // a notification tapped while the app is open (service worker)
    const fromSW = e => {
      if (e.data?.type !== 'open') return;
      if (e.data.server) selectServer(e.data.server);
      if (e.data.hash) location.hash = e.data.hash;
    };
    navigator.serviceWorker?.addEventListener('message', fromSW);
    return () => {
      clearInterval(every);
      document.removeEventListener('visibilitychange', back);
      navigator.serviceWorker?.removeEventListener('message', fromSW);
    };
  });

  // the color of the selected server
  $effect(() => applyAccent(currentServer()?.accent));

  // the moderation inbox of the staff, for the badge of its tab and of the bell
  $effect(() => {
    const sid = app.serverId, p = permsOf(app.me, sid);
    if (!sid || !(p.has('reports') || p.has('private'))) return;
    loadInbox(sid);
    const every = setInterval(() => { if (!document.hidden) loadInbox(sid); }, 60000);
    return () => clearInterval(every);
  });

  // Modération without the rights (a visitor): back to the live screen. Matchs is public, also without its tab.
  $effect(() => {
    if (app.me && route.name === 'moderation' && !tabs.includes(route.name)) go('direct');
  });
</script>

<svelte:window onkeydown={shortcut} />
<BottomNav {tabs} />
<div class="col">
  <Header onservers={() => (serversOpen = true)} onsearch={() => (searchOpen = true)} />
  <main>
    {#if app.error && !app.servers.length}
      <section class="card err">
        <p>{t('Impossible de joindre le monitor.')}</p>
        <button class="btn" onclick={loadServers}>{t('Réessayer')}</button>
      </section>
    {:else if route.name === 'serveurs'}
      <Servers />
    {:else if route.name === 'plus'}
      <More />
    {:else if route.name === 'joueurs'}
      <Players />
    {:else if route.name === 'joueur' && route.params[0]}
      <Player />
    {:else if route.name === 'moi'}
      <Claim />
    {:else if route.name === 'classements'}
      <Rankings />
    {:else if route.name === 'matchs'}
      <Matches />
    {:else if route.name === 'match' && route.params[0]}
      <Match />
    {:else if route.name === 'duel'}
      <Duel />
    {:else if route.name === 'stats'}
      <Stats />
    {:else if route.name === 'moderation'}
      <Moderation />
    {:else if route.name === 'personnalisation'}
      <Brand />
    {:else if route.name === 'equipe'}
      <Team />
    {:else if route.name === 'console'}
      <Console />
    {:else if route.name === 'programmes'}
      <Schedules />
    {:else if route.name === 'journal'}
      <Journal />
    {:else if route.name === 'motdepasse'}
      <Password />
    {:else if route.name === 'etat'}
      <Status />
    {:else if route.name === 'reglages'}
      <ServerSettings />
    {:else if route.name === 'notifications'}
      <Notifications />
    {:else if route.name === 'connexion'}
      <Login />
    {:else if route.name === 'invitation'}
      <Invitation />
    {:else if route.name === 'comptes'}
      <Accounts />
    {:else if route.name === 'bans'}
      <Bans />
    {:else if route.name === 'chat'}
      <Chat />
    {:else if route.name === 'b3'}
      <B3 />
    {:else if route.name === 'contournements'}
      <Evasions />
    {:else if route.name === 'comptes-lies'}
      <Linked />
    {:else if route.name === 'demos'}
      <Demos />
    {:else}
      <Direct />
    {/if}
  </main>
</div>
<ServerSheet open={serversOpen} onclose={() => (serversOpen = false)} />
<SearchPalette open={searchOpen} onclose={() => (searchOpen = false)} />
<Toast />

<style>
  main { padding: 16px calc(16px + var(--sar)) calc(var(--nav-h) + 40px + env(safe-area-inset-bottom)) calc(16px + var(--sal)); max-width: 1280px; }
  .err { padding: 16px; display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
  .err p { margin: 0; }
  @media (min-width: 960px) {
    .col { padding-left: 240px; }
    main { padding: 24px 28px 48px; }
  }
</style>
