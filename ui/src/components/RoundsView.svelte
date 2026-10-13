<script>
  // Rounds of the period (rounds without kills hidden): averages by map, then the latest rounds.
  import CodName from './CodName.svelte';
  import { get } from '../lib/api.js';
  import { mapName } from '../lib/state.svelte.js';
  import { dateTime, dur, fmt } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ sid: number, period: string }} */
  let { sid, period } = $props();

  let d = $state(/** @type {any} */ (null));
  let failed = $state(false);

  $effect(() => {
    const s = sid, p = period;
    if (!s) return;
    let alive = true;
    d = null;
    failed = false;
    get(`/api/servers/${s}/rounds?range=${p}`).then(r => { if (alive) d = r; }).catch(() => { if (alive) { d = { maps: [], recent: [] }; failed = true; } });
    return () => { alive = false; };
  });
</script>

{#if d === null}
  <p class="muted pad">{t('Chargement…')}</p>
{:else if failed}
  <p class="muted pad">{t('Rounds indisponibles.')}</p>
{:else}
  <div class="grid">
    <section class="card box" aria-label={t('Par map')}>
      <h2 class="label">{t('Par map')}</h2>
      <div class="tr th" aria-hidden="true"><span>{t('Map')}</span><span>{t('Rounds')}</span><span>{t('Durée moy.')}</span><span>{t('Kills moy.')}</span></div>
      {#each d.maps ?? [] as m (m.map)}
        <div class="tr"><span class="mp">{mapName(m.map)}</span><span>{fmt(m.rounds)}</span><span>{dur(m.avg_duration_s)}</span><span>{(m.avg_kills ?? 0).toFixed(1)}</span></div>
      {:else}
        <p class="muted empty">{t('Aucun round avec des kills')}</p>
      {/each}
    </section>

    <section class="card box" aria-label={t('Derniers rounds')}>
      <h2 class="label">{t('Derniers rounds')}</h2>
      {#each d.recent ?? [] as r (r.id)}
        <div class="rr">
          <span class="l">
            <b>{mapName(r.map)} <small class="muted">{(r.gametype || '').toUpperCase()}</small></b>
            <span class="muted">{dateTime(r.started_at)} · {r.ended_at ? dur(r.ended_at - r.started_at) : t('en cours')}</span>
          </span>
          <span class="top">
            {#if r.top_killer}<span class="pn"><CodName raw={r.top_killer} /></span><span class="muted">{t('meilleur tueur · {n}', { n: r.top_kills })}</span>{/if}
          </span>
          <span class="k"><b>{r.kills}</b><small>kills</small></span>
        </div>
      {:else}
        <p class="muted empty">{t('Aucun round avec des kills')}</p>
      {/each}
    </section>
  </div>
{/if}

<style>
  .pad { margin-top: 14px; }
  .grid { display: grid; gap: 12px; margin-top: 14px; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; min-width: 0; }
  .box h2 { margin-bottom: 6px; }
  .tr { display: grid; grid-template-columns: minmax(0, 1.6fr) repeat(3, minmax(0, 1fr)); gap: 8px; align-items: center; min-height: 40px; border-top: 1px solid var(--line-soft); font-size: 14px; font-variant-numeric: tabular-nums; }
  .tr span:not(:first-child) { text-align: right; }
  .tr.th { min-height: 28px; border-top: 0; font-size: 11px; font-weight: 800; color: var(--muted); }
  .mp { font-weight: 700; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .rr { display: flex; align-items: center; gap: 10px; min-height: 56px; padding: 6px 0; border-top: 1px solid var(--line-soft); }
  .rr .l { flex: 1.3; min-width: 0; display: flex; flex-direction: column; }
  .rr .l b { font-size: 14px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .rr .l small { font-size: 11px; }
  .rr .l .muted, .top .muted { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .top { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .top .pn { display: flex; min-width: 0; overflow: hidden; white-space: nowrap; font-size: 13px; font-weight: 700; }
  .k { display: flex; flex-direction: column; align-items: flex-end; width: 44px; flex: none; }
  .k b { font-family: var(--font-display); font-size: 18px; }
  .k small { font-size: 11px; color: var(--muted); }
  .empty { margin: 8px 0 0; }
  @media (min-width: 960px) {
    .grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1.6fr); align-items: start; }
  }
</style>
