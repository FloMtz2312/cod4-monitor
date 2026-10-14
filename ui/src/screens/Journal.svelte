<script>
  // Journal d'activité: what the accounts did from the site (bans, kicks, rcon, settings…), newest first.
  // The super admin sees every server, an administrator the servers they own.
  import BackTitle from '../components/BackTitle.svelte';
  import { get } from '../lib/api.js';
  import { app } from '../lib/state.svelte.js';
  import { agoText } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  let log = $state(null);
  let failed = $state('');
  let query = $state('');
  let shown = $state(60);

  $effect(() => {
    if (!app.me || app.me.role === 'visitor') return;
    get('/api/audit').then(r => { log = r.log ?? []; }).catch(e => { log = []; failed = e.message; });
  });

  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return (log ?? []).filter(l => !q || [l.username, l.action, l.detail, l.server].some(x => String(x ?? '').toLowerCase().includes(q)));
  });
  const when = ts => new Date(ts * 1000).toLocaleString(undefined, { dateStyle: 'short', timeStyle: 'short' });
</script>

<BackTitle title={t('Journal d\'activité')} sub={t('Qui a fait quoi depuis le site (300 dernières actions).')} />

{#if failed}
  <p class="muted">{failed}</p>
{:else if log === null}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <label class="search">
    <span class="sr-only">{t('Rechercher')}</span>
    <input type="search" bind:value={query} placeholder={t('Compte, action, joueur…')} autocomplete="off" />
  </label>
  <div class="card rows">
    {#each rows.slice(0, shown) as l, i (i)}
      <div class="row">
        <span class="top"><b>{t(l.action)}</b><span class="muted" title={when(l.ts)}>{agoText(l.ts)}</span></span>
        {#if l.detail}<span class="detail">{l.detail}</span>{/if}
        <span class="muted small">{l.username || t('admin')}{#if l.server}{' · '}{l.server.replace(/\^\d/g, '')}{/if}</span>
      </div>
    {:else}
      <p class="muted empty">{t('Rien pour le moment.')}</p>
    {/each}
  </div>
  {#if rows.length > shown}<button class="btn more" onclick={() => (shown += 60)}>{t('Afficher plus ({n} restants)', { n: rows.length - shown })}</button>{/if}
{/if}

<style>
  .search input { width: 100%; height: 46px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface); padding: 0 14px; font-size: 16px; margin-bottom: 12px; }
  .rows { overflow: hidden; }
  .row { display: flex; flex-direction: column; gap: 2px; padding: 10px 14px; border-top: 1px solid var(--line-soft); }
  .row:first-child { border-top: 0; }
  .top { display: flex; justify-content: space-between; gap: 10px; }
  .top .muted { font-size: 12px; white-space: nowrap; }
  .detail { font-size: 14px; color: var(--text-2); overflow-wrap: anywhere; }
  .small { font-size: 12px; }
  .empty { margin: 0; padding: 14px; }
  .more { width: 100%; margin-top: 12px; }
</style>
