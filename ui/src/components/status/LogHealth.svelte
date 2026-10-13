<script>
  // Account admins: collection of the server's log files (FTP) and the state of the backups.
  import { get } from '../../lib/api.js';
  import { app } from '../../lib/state.svelte.js';
  import { ago, fmt } from '../../lib/format.js';
  import { t } from '../../lib/i18n.svelte.js';
  import { dt } from '../stats/util.js';

  /** @type {{ serverId: number }} */
  let { serverId } = $props();

  let d = $state(null);
  let failed = $state('');
  $effect(() => {
    let alive = true;
    get('/api/observability').then(r => { if (alive) d = r; }).catch(e => { if (alive) failed = e.message || 'error'; });
    return () => { alive = false; };
  });

  const logs = $derived((d?.logs ?? []).filter(r => String(r.server_id) === String(serverId)));
  const superAdmin = $derived(app.me?.role === 'superadmin' || !!app.me?.noauth);
  const kb = b => (b == null ? '—' : b < 1024 ? `${b} o` : b < 1048576 ? `${(b / 1024).toFixed(0)} Ko` : `${(b / 1048576).toFixed(1)} Mo`);
</script>

<section class="card s-box s-wide" aria-label={t('Collecte des logs et sauvegardes')}>
  <h2 class="label">{t('Collecte des logs et sauvegardes')}</h2>
  {#if failed}
    <p class="muted">{failed}</p>
  {:else if !d}
    <p class="muted">{t('Chargement…')}</p>
  {:else}
    <p class="small">
      {#if d.backup_verified_at}
        <span class="ok">✓</span> {t('Dernière restauration vérifiée : {date}', { date: dt(Number(d.backup_verified_at)) })}
      {:else if superAdmin}
        <span class="muted">{t('Aucune restauration de sauvegarde vérifiée pour le moment.')}</span>
      {:else}
        <span class="muted">{t('La sauvegarde globale est gérée par le super admin.')}</span>
      {/if}
      {#if d.backup_error}<span class="bad">{' · ⚠ '}{d.backup_error}</span>{/if}
    </p>
    {#if logs.length}
      <div class="s-scroll">
        <table class="s-table">
          <thead><tr><th>{t('Flux')}</th><th>{t('Dernière lecture')}</th><th class="n">{t('Retard')}</th><th class="n">{t('Lignes')}</th><th class="n">{t('Rejets')}</th><th class="n">{t('Erreurs')}</th><th class="n">{t('Rotations')}</th></tr></thead>
          <tbody>
            {#each logs as r (r.file)}
              <tr>
                <td><b>{r.file}</b>{#if r.missing}<span class="bad">{' ⚠ '}{t('absent')}</span>{/if}</td>
                <td>{r.last_success ? t('il y a {d}', { d: ago(r.last_success) }) : '—'}</td>
                <td class="n" class:bad={r.backlog_bytes > 1048576}>{kb(r.backlog_bytes)}</td>
                <td class="n">{fmt(r.lines)}</td>
                <td class="n" class:bad={r.rejected > 0}>{fmt(r.rejected)}</td>
                <td class="n" class:bad={r.errors > 0}>{fmt(r.errors)}</td>
                <td class="n">{fmt(r.rotations)}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {:else}
      <p class="muted">{t('Aucun flux de logs configuré.')}</p>
    {/if}
  {/if}
</section>

<style>
  .small { font-size: 13px; }
  .ok { color: var(--ok); font-weight: 800; }
  .bad { color: var(--danger-text); font-weight: 700; }
</style>
