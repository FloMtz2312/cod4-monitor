<script>
  // Tous les serveurs: pick one; an administrator also sees the servers they manage (owner, address, logs, rcon, state)
  // with shortcuts to their settings and team, adds a server, and finds the servers whose monitoring is stopped.
  import BackTitle from '../components/BackTitle.svelte';
  import ServerRow from '../components/ServerRow.svelte';
  import Icon from '../components/Icon.svelte';
  import { get } from '../lib/api.js';
  import { app, selectServer } from '../lib/state.svelte.js';
  import { go, href } from '../lib/router.svelte.js';
  import { clean } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  const admin = $derived(['admin', 'superadmin'].includes(app.me?.role) || !!app.me?.noauth);
  let owned = $state([]);
  $effect(() => {
    if (!admin) return;
    get('/api/serverconf').then(r => { owned = r.servers ?? []; }).catch(() => {});
  });
  const mine = $derived(new Set(owned.map(s => s.id)));
  const live = $derived(new Map(app.servers.map(s => [s.id, s])));

  function stateOf(c) {
    if (!c.managed) return ['', t('historique seulement')];
    if (!c.enabled) return ['', t('arrêté')];
    const l = live.get(c.id);
    if (!l) return ['', t('démarrage…')];
    return l.online ? ['on', t('en ligne')] : ['off', t('hors ligne')];
  }
  function team(c) {
    selectServer(c.id);
    go('equipe');
  }
</script>

<BackTitle title={t('Tous les serveurs')} fallback="plus" />
<div class="list">
  {#each app.servers as s (s.id)}
    <div class="item">
      <ServerRow server={s} selected={s.id === app.serverId} onpick={() => { selectServer(s.id); go('direct'); }} />
      {#if mine.has(s.id)}<a class="cfg" href={href('reglages', String(s.id))} aria-label={t('Réglages de {name}', { name: clean(s.name) })}><Icon name="settings" size={20} /></a>{/if}
    </div>
  {:else}
    <p class="muted">{t('Aucun serveur surveillé pour le moment.')}</p>
  {/each}
</div>

{#if admin && owned.length}
  <h2 class="label sec">{t('Gestion des serveurs')}</h2>
  <div class="grid">
    {#each owned as c (c.id)}
      {@const st = stateOf(c)}
      <section class="card conf">
        <div class="ch">
          <b>{clean(c.name)}</b>
          <span class="st"><span class="dot {st[0]}"></span>{st[1]}</span>
        </div>
        <dl>
          <dt>{t('Propriétaire')}</dt><dd>{c.owner || t('Super admin (.env)')}</dd>
          <dt>{t('Adresse')}</dt><dd class="mono">{c.address}</dd>
          <dt>{t('Logs')}</dt>
          <dd>{#if c.log_dir}{t('sur cette machine')} · <span class="mono">{c.log_dir}</span>{:else if c.ftp}FTP · <span class="mono">{c.ftp.host}{c.ftp.path}</span>{:else}<span class="muted">{t('aucun')}</span>{/if}</dd>
          <dt>RCON</dt><dd>{c.has_rcon ? t('oui') : t('non')}</dd>
        </dl>
        <div class="acts">
          <a class="btn" href={href('reglages', String(c.id))}>{c.managed ? t('Réglages') : t('Configurer')}</a>
          {#if live.has(c.id)}<button class="btn" onclick={() => team(c)}>{t('Équipe')}</button>{/if}
        </div>
      </section>
    {/each}
  </div>
{/if}

{#if admin}
  <a class="btn add" href={href('reglages', 'nouveau')}>+ {t('Ajouter un serveur')}</a>
  <p class="muted note">{t('Les changements s\'appliquent tout de suite, sans redémarrer le monitor. Arrêter un serveur garde tout son historique.')}</p>
{/if}

<style>
  .list { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr)); gap: 10px; }
  .item { display: flex; gap: 8px; align-items: stretch; min-width: 0; }
  .item :global(.row) { flex: 1; min-width: 0; }
  .cfg { width: 48px; flex: none; border-radius: 16px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); display: flex; align-items: center; justify-content: center; }
  .sec { display: block; margin: 18px 2px 8px; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(min(100%, 340px), 1fr)); gap: 10px; }
  .conf { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .ch { display: flex; justify-content: space-between; align-items: center; gap: 10px; }
  .ch b { min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .st { display: inline-flex; align-items: center; gap: 6px; font-size: 12px; color: var(--text-2); font-weight: 700; white-space: nowrap; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--muted); }
  .dot.on { background: var(--ok); }
  .dot.off { background: var(--danger); }
  dl { margin: 0; display: grid; grid-template-columns: auto 1fr; gap: 4px 12px; font-size: 13px; }
  dt { color: var(--muted); }
  dd { margin: 0; min-width: 0; overflow-wrap: anywhere; }
  .mono { font-family: ui-monospace, monospace; font-size: 12px; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; }
  .acts .btn { min-height: 40px; padding: 0 14px; font-size: 14px; }
  .add { width: 100%; margin-top: 16px; }
  .note { font-size: 12px; margin: 8px 2px 0; }
</style>
