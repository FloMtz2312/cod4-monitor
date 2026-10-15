<script>
  import Icon from './Icon.svelte';
  import { tabInfo } from '../lib/roles.js';
  import { route } from '../lib/router.svelte.js';
  import { app } from '../lib/state.svelte.js';
  import { inboxCount } from '../lib/inbox.svelte.js';
  import { chatMentions, chatUnread } from '../lib/adminchat.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ tabs: string[] }} */
  let { tabs } = $props();
  // screens opened from a tab keep it lit
  const parent = {
    joueur: 'joueurs', moi: 'joueurs', match: 'matchs',
    serveurs: 'plus', personnalisation: 'plus', equipe: 'plus', console: 'plus', programmes: 'plus', journal: 'plus',
    motdepasse: 'plus', etat: 'plus', reglages: 'plus', notifications: 'plus',
    connexion: 'plus', invitation: 'plus', comptes: 'plus', 'logs-monitor': 'plus',
    bans: 'moderation', chat: 'moderation', 'tchat-admins': 'moderation', b3: 'moderation', contournements: 'moderation', 'comptes-lies': 'moderation', demos: 'moderation'
  };
  // screens linked from several tabs keep lit the tab they were opened from (Stats from Plus lights Plus)
  const from = { stats: ['classements', 'plus'], duel: ['classements', 'joueurs'], matchs: ['classements', 'plus'], match: ['classements', 'plus'] };
  let last = $state('direct');
  $effect(() => {
    const p = tabs.includes(route.name) ? route.name : parent[route.name] ?? route.name;
    if (!from[route.name] && tabs.includes(p)) last = p;
  });
  // mentions in the admins' chat: on its tab when it has one, else on Modération
  const chat = $derived(tabs.includes('tchat-admins'));
  const todo = $derived(inboxCount(app.serverId) + (chat ? 0 : chatMentions(app.serverId)));
  const mentions = $derived(chatMentions(app.serverId));
  const unread = $derived(chatUnread(app.serverId));
  const active = $derived.by(() => {
    const p = tabs.includes(route.name) ? route.name : parent[route.name] ?? route.name;
    if (tabs.includes(p)) return p;
    const opts = (from[route.name] ?? []).filter(x => tabs.includes(x));
    // otherwise (Matchs for the staff…) Classements, where its link is
    return opts.includes(last) ? last : opts[0] ?? 'classements';
  });
</script>

<!-- Floating bar at the bottom of a phone; a side menu from 960 px. -->
<nav aria-label={t('Navigation principale')} class:many={tabs.length > 5}>
  <div class="brand"><span>CoD4 Monitor</span></div>
  {#each tabs as tab (tab)}
    <a href={'#/' + tab} class:on={active === tab} aria-current={active === tab ? 'page' : undefined}>
      <span class="pill"><Icon name={tabInfo[tab].icon} />{#if tab === 'moderation' && todo}<span class="badge" aria-label={t('{n} à traiter', { n: todo })}>{todo > 9 ? '9+' : todo}</span>{/if}{#if tab === 'tchat-admins' && (mentions || unread)}<span class="badge" class:soft={!mentions} aria-label={mentions ? t('{n} mentions', { n: mentions }) : t('{n} non lus', { n: unread })}>{mentions ? '@' + (mentions > 9 ? '9+' : mentions) : unread > 9 ? '9+' : unread}</span>{/if}</span>
      <span class="lbl">{t(tabInfo[tab].label)}</span>
    </a>
  {/each}
</nav>

<style>
  nav {
    position: fixed; left: calc(12px + var(--sal)); right: calc(12px + var(--sar)); bottom: calc(14px + env(safe-area-inset-bottom)); z-index: 10; height: var(--nav-h);
    display: flex; align-items: stretch; padding: 0 4px; border-radius: 24px;
    background: rgba(20, 25, 34, .94); border: 1px solid #2A3446; box-shadow: 0 10px 30px rgba(0, 0, 0, .45);
    backdrop-filter: blur(12px); -webkit-backdrop-filter: blur(12px);
  }
  .brand { display: none; }
  a {
    flex: 1; min-width: 0; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 3px;
    color: var(--muted); font-size: 11px; font-weight: 700; text-decoration: none;
  }
  .pill { position: relative; width: 52px; height: 30px; border-radius: 15px; display: flex; align-items: center; justify-content: center; transition: background .15s; }
  .badge { position: absolute; top: -4px; right: 4px; min-width: 18px; height: 18px; padding: 0 5px; border-radius: 9px; background: var(--danger); color: #fff; font-size: 11px; font-weight: 800; display: flex; align-items: center; justify-content: center; }
  .badge.soft { background: var(--surface-2); color: var(--text); border: 1px solid var(--line); }
  .lbl { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; max-width: 100%; }
  @media (max-width: 959px) { .many a { font-size: 10px; } .many .pill { width: 46px; } } /* six tabs on a phone */
  a.on { color: var(--accent); }
  a.on .pill { background: var(--accent-soft); }

  @media (min-width: 960px) {
    nav {
      left: 0; top: 0; bottom: 0; right: auto; width: 240px; height: auto; flex-direction: column; align-items: stretch; gap: 4px;
      padding: calc(20px + var(--sat)) 16px 20px; border-radius: 0; border: 0; border-right: 1px solid var(--surface-2); background: var(--bg); box-shadow: none;
    }
    .brand { display: block; font-family: var(--font-display); font-weight: 700; font-size: 17px; padding: 8px 12px 20px; color: var(--text); }
    a { flex: none; flex-direction: row; justify-content: flex-start; gap: 10px; height: 44px; padding: 0 12px; border-radius: 12px; font-size: 15px; color: var(--text-2); }
    .pill { width: auto; height: auto; background: none !important; }
    a.on { background: var(--accent-soft); }
  }
</style>
