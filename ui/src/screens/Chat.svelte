<script>
  // Chat history of the server (private data), newest first: filter on a player (any of their names, or every name
  // of a GUID with #/chat/<guid>), search a text, load more.
  import BackTitle from '../components/BackTitle.svelte';
  import CodName from '../components/CodName.svelte';
  import { get } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { route, href, replace } from '../lib/router.svelte.js';
  import { clean, colorParts } from '../lib/format.js';
  import { dt } from '../lib/moderation.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const srv = $derived(currentServer());
  const allowed = $derived(permsOf(app.me, id).has('private'));
  const key = $derived(route.params[0] ?? '');
  const guid = $derived(key && !key.startsWith('n:') ? key : '');

  let name = $state('');
  let q = $state('');
  let dq = $state({ name: '', q: '' }); // debounced
  let limit = $state(100);
  let messages = $state(null);
  let more = $state(false);

  // #/chat/n:<name> fills the player field; another link clears the filters
  $effect(() => {
    const n = key.startsWith('n:') ? key.slice(2) : '';
    name = n;
    q = '';
    dq = { name: n, q: '' };
    limit = 100;
  });

  let timer;
  function typed() {
    clearTimeout(timer);
    timer = setTimeout(() => { dq = { name: name.trim(), q: q.trim() }; limit = 100; }, 300);
  }

  $effect(() => {
    const sid = id, g = guid, f = dq, n = limit;
    if (!sid || !allowed) return;
    let alive = true;
    const p = new URLSearchParams(g ? { guid: g, q: f.q, limit: String(n) } : { name: f.name, q: f.q, limit: String(n) });
    get(`/api/servers/${sid}/chat?${p}`).then(r => {
      if (!alive) return;
      messages = r.messages ?? [];
      more = messages.length >= n;
    }).catch(() => { if (alive) messages = []; });
    return () => { alive = false; };
  });

  const guidName = $derived(guid && messages?.length ? messages[0].name_raw : '');
  const whoKey = m => m.guid || 'n:' + clean(m.name_raw);
</script>

<BackTitle title={t('Chat')} sub={srv ? clean(srv.name) : ''} fallback="moderation" />

{#if !allowed}
  <p class="muted">{t('Il faut l\'accès aux données privées de ce serveur.')}</p>
{:else}
  <div class="bar">
    {#if guid}
      <span class="chip">{t('Joueur')} : {#if guidName}<CodName raw={guidName} />{:else}<code>{guid}</code>{/if}
        <button aria-label={t('Retirer le filtre')} onclick={() => replace('chat')}>×</button></span>
    {:else}
      <label class="sr-only" for="chat-name">{t('Joueur')}</label>
      <input id="chat-name" bind:value={name} oninput={typed} placeholder={t('Joueur (tous ses pseudos)')} autocomplete="off" autocapitalize="none" spellcheck="false" />
    {/if}
    <label class="sr-only" for="chat-q">{t('Rechercher')}</label>
    <input id="chat-q" bind:value={q} oninput={typed} placeholder={t('Rechercher un message…')} autocomplete="off" spellcheck="false" />
  </div>

  {#if messages === null}
    <p class="muted">{t('Chargement…')}</p>
  {:else}
    <div class="card rows">
      {#each messages as m, i (i)}
        <div class="msg">
          <div class="meta">
            <a href={href('chat', whoKey(m))} title={t('Tout son chat')}><CodName raw={m.name_raw} /></a>
            {#if m.team}<span class="team">{t('équipe')}</span>{/if}
            <span class="muted when">{dt(m.ts)}</span>
          </div>
          <p>{#each colorParts(m.message) as p, j (j)}<span style:color={p.color}>{p.text}</span>{/each}</p>
        </div>
      {:else}
        <p class="muted empty">{t('Aucun message')}</p>
      {/each}
    </div>
    {#if more}<button class="btn moreb" onclick={() => (limit += 200)}>{t('Afficher plus')}</button>{/if}
  {/if}
{/if}

<style>
  .bar { display: flex; flex-wrap: wrap; gap: 8px; margin-bottom: 12px; }
  .bar input { flex: 1 1 220px; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); padding: 0 12px; font-size: 16px; }
  .chip { display: inline-flex; align-items: center; gap: 6px; min-height: 44px; padding: 0 6px 0 12px; border-radius: 12px; border: 1px solid var(--accent); background: var(--accent-soft); font-weight: 700; font-size: 14px; max-width: 100%; }
  .chip code { font-size: 12px; overflow: hidden; text-overflow: ellipsis; }
  .chip button { width: 32px; height: 32px; border: 0; border-radius: 9px; background: transparent; color: var(--text); font-size: 20px; line-height: 1; }
  .rows { overflow: hidden; }
  .msg { padding: 10px 16px; border-top: 1px solid var(--line-soft); min-width: 0; }
  .msg:first-child { border-top: 0; }
  .meta { display: flex; align-items: center; gap: 8px; min-width: 0; font-size: 13px; font-weight: 700; }
  .meta a { color: var(--text); text-decoration: none; display: inline-flex; min-width: 0; }
  .team { font-size: 11px; font-weight: 800; padding: 1px 6px; border-radius: 6px; background: var(--surface-2); color: var(--text-2); flex: none; }
  .when { margin-left: auto; font-size: 12px; font-weight: 500; white-space: nowrap; }
  .msg p { margin: 3px 0 0; font-size: 14px; color: var(--text-2); overflow-wrap: anywhere; }
  .empty { margin: 0; padding: 14px 16px; }
  .moreb { width: 100%; margin-top: 12px; }
</style>
