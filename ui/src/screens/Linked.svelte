<script>
  // Comptes liés (private data), two lists: every name used by each GUID (with its IPs), and the IPs shared by
  // several GUIDs (alternate accounts or ban evasion). A row opens the player's profile.
  import BackTitle from '../components/BackTitle.svelte';
  import Seg from '../components/Seg.svelte';
  import CodName from '../components/CodName.svelte';
  import { get } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { href } from '../lib/router.svelte.js';
  import { agoText, clean } from '../lib/format.js';
  import { dt } from '../lib/moderation.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const srv = $derived(currentServer());
  const allowed = $derived(permsOf(app.me, id).has('private'));
  let tab = $state('names');
  let aliases = $state(null);
  let alts = $state(null);
  let filter = $state('');
  let max = $state(100);

  $effect(() => {
    const sid = id;
    if (!sid || !allowed) return;
    let alive = true;
    aliases = alts = null;
    get(`/api/servers/${sid}/aliases`).then(r => { if (alive) aliases = r.aliases ?? []; }).catch(() => { if (alive) aliases = []; });
    get(`/api/servers/${sid}/alts`).then(r => { if (alive) alts = r.alts ?? []; }).catch(() => { if (alive) alts = []; });
    return () => { alive = false; };
  });

  // one entry per GUID, its names from the most recent
  const byGuid = $derived.by(() => {
    const m = new Map();
    for (const a of aliases ?? []) {
      if (!m.has(a.guid)) m.set(a.guid, []);
      m.get(a.guid).push(a);
    }
    return [...m.entries()].map(([guid, names]) => ({ guid, names, ips: (names[0].ips ?? '').split(',').filter(Boolean), last: Math.max(...names.map(n => n.last_seen)) }));
  });
  const shownNames = $derived.by(() => {
    const f = filter.trim().toLowerCase();
    return f ? byGuid.filter(g => (g.guid + ' ' + g.names.map(n => clean(n.name_raw)).join(' ') + ' ' + g.ips.join(' ')).toLowerCase().includes(f)) : byGuid;
  });
  $effect(() => { filter; max = 100; });

  const byIP = $derived.by(() => {
    const m = new Map();
    for (const x of alts ?? []) {
      if (!m.has(x.ip)) m.set(x.ip, []);
      m.get(x.ip).push(x);
    }
    return [...m.entries()];
  });
</script>

<BackTitle title={t('Comptes liés')} sub={srv ? clean(srv.name) : ''} fallback="moderation" />

{#if !allowed}
  <p class="muted">{t('Il faut l\'accès aux données privées de ce serveur.')}</p>
{:else}
  <div class="tabs"><Seg label={t('Vue')} value={tab} onchange={v => (tab = v)} items={[['names', t('Pseudos par GUID')], ['ips', t('Même IP')]]} /></div>

  {#if tab === 'names'}
    <label class="sr-only" for="alias-filter">{t('Filtrer')}</label>
    <input id="alias-filter" class="filter" bind:value={filter} placeholder={t('Pseudo, GUID ou IP…')} autocomplete="off" autocapitalize="none" spellcheck="false" />
    {#if aliases === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else}
      <div class="card rows">
        {#each shownNames.slice(0, max) as g (g.guid)}
          <a class="row" href={href('joueur', g.guid)}>
            <span class="names">
              {#each g.names as n, i (i)}<span title={t('vu du {a} au {b}', { a: dt(n.first_seen), b: dt(n.last_seen) })}><CodName raw={n.name_raw} /></span>{/each}
              {#if g.names.length > 1}<b class="count">{g.names.length}</b>{/if}
            </span>
            <span class="muted sub"><code>{g.guid}</code>{#if g.ips.length}{' · '}{g.ips.join(', ')}{/if}{' · '}{agoText(g.last)}</span>
          </a>
        {:else}
          <p class="muted empty">{t('Aucun GUID connu (nécessite les logs du serveur)')}</p>
        {/each}
      </div>
      {#if shownNames.length > max}<button class="btn moreb" onclick={() => (max += 200)}>{t('Afficher plus ({n} restants)', { n: shownNames.length - max })}</button>{/if}
    {/if}
  {:else if alts === null}
    <p class="muted">{t('Chargement…')}</p>
  {:else}
    <p class="muted intro">{t('IP utilisées par plusieurs GUID : comptes secondaires ou contournement de ban.')}</p>
    <div class="grid">
      {#each byIP as [ip, xs] (ip)}
        <section class="card ipbox">
          <h2 class="ip"><code>{ip}</code> <span class="muted">· {t('{n} comptes', { n: xs.length })}</span></h2>
          {#each xs as x (x.key)}
            <a class="acc" href={href('joueur', x.key)}><CodName raw={x.name_raw || x.key} /><span class="muted">{t('{n} sessions', { n: x.sessions })}</span></a>
          {/each}
        </section>
      {:else}
        <section class="card ipbox"><p class="muted">{t('Aucune IP partagée entre plusieurs GUID')}</p></section>
      {/each}
    </div>
  {/if}
{/if}

<style>
  .tabs { margin-bottom: 12px; max-width: 420px; }
  .filter { width: 100%; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); padding: 0 12px; font-size: 16px; margin-bottom: 12px; }
  .rows { overflow: hidden; }
  .row { display: flex; flex-direction: column; gap: 3px; padding: 10px 16px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); min-width: 0; }
  .row:first-child { border-top: 0; }
  .names { display: flex; flex-wrap: wrap; gap: 4px 10px; align-items: center; font-weight: 700; min-width: 0; }
  .names > span { display: inline-flex; min-width: 0; max-width: 100%; }
  .count { font-size: 12px; color: var(--accent); }
  .sub { font-size: 12px; overflow-wrap: anywhere; }
  .sub code { font-size: 11px; }
  .empty { margin: 0; padding: 14px 16px; }
  .moreb { width: 100%; margin-top: 12px; }
  .intro { margin: 0 2px 10px; font-size: 13px; }
  .grid { display: grid; gap: 10px; grid-template-columns: repeat(auto-fill, minmax(min(100%, 300px), 1fr)); align-items: start; }
  .ipbox { padding: 12px 14px; display: flex; flex-direction: column; gap: 6px; min-width: 0; }
  .ipbox p { margin: 0; }
  .ip { font-size: 14px; font-weight: 700; }
  .acc { display: flex; justify-content: space-between; gap: 10px; align-items: center; min-height: 36px; padding-top: 6px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); font-weight: 700; min-width: 0; }
  .acc .muted { font-size: 12px; font-weight: 500; white-space: nowrap; }
</style>
