<script>
  // Recherche (Ctrl K, or / on a computer): the players of the selected server (current and old names) and the screens
  // of the app the account may open. Arrows to move, Enter to open.
  import Sheet from './Sheet.svelte';
  import Icon from './Icon.svelte';
  import CodName from './CodName.svelte';
  import { get } from '../lib/api.js';
  import { app, role } from '../lib/state.svelte.js';
  import { permsOf, isOwner } from '../lib/perms.js';
  import { go } from '../lib/router.svelte.js';
  import { clean } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ open: boolean, onclose: () => void }} */
  let { open, onclose } = $props();

  // [route, label, icon, who may open it]
  /** @type {[string, string, string, (p: Set<string>, owner: boolean, staff: boolean) => boolean][]} */
  const SCREENS = [
    ['direct', 'Direct', 'live', () => true],
    ['joueurs', 'Joueurs', 'users', () => true],
    ['classements', 'Classements', 'trophy', () => true],
    ['matchs', 'Matchs récents', 'matches', () => true],
    ['stats', 'Stats avancées', 'chart', () => true],
    ['etat', 'État du serveur', 'pulse', () => true],
    ['serveurs', 'Tous les serveurs', 'globe', () => true],
    ['moderation', 'Modération', 'shield', p => p.has('reports') || p.has('private')],
    ['duel', 'Duel', 'users', () => true],
    ['console', 'Console', 'terminal', p => p.has('rcon') || p.has('private')],
    ['chat', 'Chat', 'message', p => p.has('private')],
    ['chat-admins', 'Chat des admins', 'message', p => p.size > 0 && !app.me?.noauth],
    ['bans', 'Bans du serveur', 'ban', p => p.has('ban')],
    ['contournements', 'Contournements de ban', 'alert', p => p.has('private')],
    ['comptes-lies', 'Comptes liés', 'link', p => p.has('private')],
    ['demos', 'Démos', 'film', p => p.has('private')],
    ['b3', 'B3', 'bot', p => p.has('private')],
    ['comptes', 'Comptes', 'key', () => !!app.me?.admin],
    ['logs-monitor', 'Logs du monitor', 'terminal', () => !!app.me?.admin],
    ['programmes', 'Commandes programmées', 'clock', p => p.has('rcon')],
    ['equipe', 'Équipe et invitations', 'users', (p, owner) => owner],
    ['personnalisation', 'Personnalisation', 'palette', (p, owner) => owner],
    ['notifications', 'Notifications', 'bell', (p, owner, staff) => staff],
    ['plus', 'Plus', 'more', () => true]
  ];

  let q = $state('');
  let sel = $state(0);
  /** @type {any[]} */
  let players = $state([]);
  let loadedFor = 0;
  /** @type {HTMLInputElement | undefined} */
  let input = $state();

  $effect(() => {
    if (!open) return;
    q = '';
    sel = 0;
    setTimeout(() => input?.focus(), 30);
    const sid = app.serverId;
    if (sid && loadedFor !== sid) {
      loadedFor = sid;
      get(`/api/servers/${sid}/players?range=90d`).then(r => { players = r.players ?? []; }).catch(() => { loadedFor = 0; });
    }
  });

  const norm = s => clean(s).toLowerCase().normalize('NFD').replace(/[̀-ͯ]/g, '');
  const results = $derived.by(() => {
    const s = norm(q.trim());
    const p = permsOf(app.me, app.serverId), owner = isOwner(app.me, app.serverId), staff = role() !== 'visitor';
    /** @typedef {{ kind: string, id: string, label: string, icon: string, raw: string, alias: string }} Result */
    /** @type {Result[]} */
    const screens = SCREENS.filter(([, label, , ok]) => ok(p, owner, staff) && (!s || norm(t(label)).includes(s)))
      .map(([r, label, icon]) => ({ kind: 'screen', id: r, label: t(label), icon, raw: '', alias: '' }));
    /** @type {Result[]} */
    const found = !s ? [] : players
      .filter(x => norm(x.name_clean ?? x.name_raw).includes(s) || norm(x.aliases ?? '').includes(s))
      .slice(0, 8)
      .map(x => ({ kind: 'player', id: x.key, label: '', icon: 'user', raw: x.name_raw ?? x.name_clean, alias: !norm(x.name_clean ?? '').includes(s) ? x.aliases ?? '' : '' }));
    return [...found, ...screens].slice(0, 14);
  });
  $effect(() => { q; sel = 0; });

  function pick(r) {
    onclose();
    if (r.kind === 'player') go('joueur', r.id);
    else go(r.id);
  }

  function keys(e) {
    if (e.key === 'ArrowDown') { e.preventDefault(); sel = Math.min(sel + 1, results.length - 1); }
    else if (e.key === 'ArrowUp') { e.preventDefault(); sel = Math.max(sel - 1, 0); }
    else if (e.key === 'Enter' && results[sel]) { e.preventDefault(); pick(results[sel]); }
  }
</script>

<Sheet {open} {onclose} label={t('Rechercher')}>
  <div class="box">
    <label class="field">
      <Icon name="search" size={20} />
      <span class="sr-only">{t('Rechercher')}</span>
      <input bind:this={input} bind:value={q} onkeydown={keys} placeholder={t('Joueur, ancien pseudo, écran…')} autocomplete="off" spellcheck="false"
        role="combobox" aria-expanded="true" aria-controls="search-results" aria-activedescendant={results[sel] ? `sr-${sel}` : undefined} />
      <kbd>Esc</kbd>
    </label>
    <ul id="search-results" role="listbox" aria-label={t('Résultats')}>
      {#each results as r, i (r.kind + r.id)}
        <li id={`sr-${i}`} role="option" aria-selected={i === sel}>
          <button class:on={i === sel} onclick={() => pick(r)} onmouseenter={() => (sel = i)} tabindex="-1">
            {#if r.kind === 'player'}
              <Icon name="user" size={18} /><span class="lbl"><CodName raw={r.raw} />{#if r.alias}<small>{clean(r.alias)}</small>{/if}</span><span class="kind">{t('Joueur')}</span>
            {:else}
              <Icon name={r.icon} size={18} /><span class="lbl">{r.label}</span><span class="kind">{t('Écran')}</span>
            {/if}
          </button>
        </li>
      {:else}
        <li class="none muted">{t('Aucun résultat')}</li>
      {/each}
    </ul>
    <p class="hint muted">{t('↑ ↓ pour choisir, Entrée pour ouvrir · Ctrl K ou / pour rechercher')}</p>
  </div>
</Sheet>

<style>
  .box { display: flex; flex-direction: column; gap: 10px; }
  .field { display: flex; align-items: center; gap: 10px; height: 50px; padding: 0 12px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface-2); color: var(--muted); }
  .field input { flex: 1; min-width: 0; height: 100%; border: 0; background: transparent; color: var(--text); font-size: 16px; outline: none; }
  kbd { font: 700 11px var(--font); padding: 3px 6px; border-radius: 6px; border: 1px solid var(--line); color: var(--muted); }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 2px; max-height: 52dvh; overflow-y: auto; }
  li button { width: 100%; display: flex; align-items: center; gap: 12px; min-height: 46px; padding: 6px 12px; border: 0; border-radius: 12px; background: transparent; color: var(--text); text-align: left; font-weight: 700; }
  li button.on { background: var(--accent-soft); }
  li button :global(svg) { color: var(--muted); flex: none; }
  .lbl { flex: 1; min-width: 0; display: flex; flex-direction: column; overflow: hidden; }
  .lbl small { font-size: 12px; color: var(--muted); font-weight: 500; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .kind { font-size: 12px; color: var(--muted); font-weight: 600; }
  .none { padding: 12px; }
  .hint { margin: 0; font-size: 12px; text-align: center; }
  @media (pointer: coarse) { .hint { display: none; } }
</style>
