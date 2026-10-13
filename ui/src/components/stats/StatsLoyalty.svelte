<script>
  // Fidélité: new players, players who come back, regulars who stopped coming.
  import CodName from '../CodName.svelte';
  import { get } from '../../lib/api.js';
  import { go } from '../../lib/router.svelte.js';
  import { t } from '../../lib/i18n.svelte.js';
  import { dur, dayLabel, dt } from './util.js';

  /** @type {{ id: number, period: string }} */
  let { id, period } = $props();

  let r = $state(null);
  let shownNew = $state(20);
  $effect(() => {
    const sid = id, p = period;
    let alive = true;
    r = null;
    shownNew = 20;
    get(`/api/servers/${sid}/retention?range=${p}`).then(x => { if (alive) r = x; }).catch(() => { if (alive) r = {}; });
    return () => { alive = false; };
  });

  const weeks = $derived(r?.weeks ?? []);
  const wmax = $derived(Math.max(1, ...weeks.map(w => (w.new ?? 0) + (w.returning ?? 0))));
  const rate = $derived(r?.return_rate);
  const rateCol = $derived(rate == null ? 'inherit' : rate >= 0.3 ? 'var(--ok)' : rate >= 0.15 ? 'var(--warn)' : 'var(--danger-text)');
  const returnDays = $derived(r?.rules?.return_days ?? 7);
  const weekLabel = w => dayLabel(new Date(w + 'T12:00:00').getTime() / 1000);
</script>

{#if r === null}
  <p class="muted pad">{t('Chargement…')}</p>
{:else}
  <div class="s-grid">
    <section class="card s-box s-wide" aria-label={t('Fidélité des joueurs')}>
      <div class="s-hd"><h2 class="label">{t('Fidélité des joueurs')}</h2>{#if r.source === 'udp'}<span class="muted s-small">{t('(sondages : moins précis sans les logs FTP)')}</span>{/if}</div>
      <div class="s-tiles">
        <div><b>{r.active ?? '—'}</b><span>{t('joueurs actifs')}</span></div>
        <div><b>{r.new ?? '—'}</b><span>{t('nouveaux joueurs')}</span></div>
        <div><b>{r.returning ?? '—'}</b><span>{t('déjà venus avant')}</span></div>
        <div title={t('parmi les nouveaux venus il y a plus de {d} jours : {r} sur {m}', { d: returnDays, r: r.returned ?? 0, m: r.measurable ?? 0 })}>
          <b style:color={rateCol}>{rate == null ? '—' : `${Math.round(100 * rate)} %`}</b><span>{t('des nouveaux reviennent (sous {d} j)', { d: returnDays })}</span>
        </div>
        <div><b style:color={r.gone?.length ? 'var(--danger-text)' : 'inherit'}>{r.gone?.length ?? 0}</b><span>{t('réguliers partis')}</span></div>
      </div>
      {#if weeks.some(w => w.new || w.returning)}
        <div class="s-sub">
          <h3 class="label">{t('Joueurs par semaine')}</h3>
          <div class="wbars" role="img" aria-label={t('Nouveaux et déjà venus par semaine')}>
            {#each weeks as w (w.week)}
              <div class="wcol" title={`${t('Semaine du {d}', { d: weekLabel(w.week) })} · ${t('Nouveaux')} ${w.new} · ${t('Déjà venus')} ${w.returning}${w.measurable ? ' · ' + t('{r} nouveau(x) sur {m} revenu(s) sous {d} j', { r: w.returned, m: w.measurable, d: returnDays }) : ''}`}>
                <div class="stack" style:height="{(100 * ((w.new ?? 0) + (w.returning ?? 0))) / wmax}%">
                  <span class="new" style:flex={w.new ?? 0}></span><span class="ret" style:flex={w.returning ?? 0}></span>
                </div>
                <span class="wl">{weekLabel(w.week)}</span>
              </div>
            {/each}
          </div>
          <div class="s-legend"><span><i style:background="var(--warn)"></i>{t('Nouveaux')}</span><span><i style:background="var(--allies)"></i>{t('Déjà venus')}</span></div>
        </div>
      {/if}
    </section>

    <section class="card s-box" aria-label={t('Nouveaux joueurs de la période')}>
      <h2 class="label">{t('Nouveaux joueurs de la période')}</h2>
      {#if !r.new_players?.length}
        <p class="muted">{t('Aucun nouveau joueur sur la période')}</p>
      {:else}
        <div class="rows">
          <div class="row head" aria-hidden="true"><span>{t('Joueur')}</span><span class="opt">{t('Première venue')}</span><span>{t('Jours')}</span><span>{t('Temps')}</span><span title={t('revenu un autre jour dans les {d} jours suivant sa première venue', { d: returnDays })}>{t('Revenu ?')}</span></div>
          {#each r.new_players.slice(0, shownNew) as x (x.key)}
            <button class="row" onclick={() => go('joueur', x.key)}>
              <CodName raw={x.name_raw ?? ''} />
              <span class="muted opt">{dt(x.first_seen)}</span>
              <span>{x.days}</span>
              <span class="muted">{dur(x.playtime_s)}</span>
              {#if x.returned == null}<span class="muted">{t('trop tôt')}</span>{:else if x.returned}<span class="yes">✓ {t('oui')}</span>{:else}<span class="no">✕ {t('non')}</span>{/if}
            </button>
          {/each}
        </div>
        {#if r.new_players.length > shownNew}<button class="btn s-more" onclick={() => (shownNew += 50)}>{t('Afficher plus')}</button>{/if}
      {/if}
    </section>

    <section class="card s-box" aria-label={t('Réguliers qui ne viennent plus')}>
      <div class="s-hd"><h2 class="label">{t('Réguliers qui ne viennent plus')}</h2>
        {#if r.rules}<span class="muted s-small">{t('(≥ {d} jours joués, absents depuis {g} j)', { d: r.rules.regular_days, g: r.rules.gone_days })}</span>{/if}</div>
      {#if !r.gone?.length}
        <p class="muted">{t('Aucun — tous les réguliers sont encore là')}</p>
      {:else}
        <div class="rows">
          <div class="row gone head" aria-hidden="true"><span>{t('Joueur')}</span><span>{t('Jours')}</span><span>{t('Temps')}</span><span>{t('Dernière venue')}</span></div>
          {#each r.gone as x (x.key)}
            <button class="row gone" onclick={() => go('joueur', x.key)}>
              <CodName raw={x.name_raw ?? ''} />
              <span>{x.days}</span>
              <span class="muted">{dur(x.playtime_s)}</span>
              <span class="muted">{dayLabel(x.last_seen)}</span>
            </button>
          {/each}
        </div>
      {/if}
    </section>
  </div>
{/if}

<style>
  .pad { margin-top: 14px; }
  .wbars { display: flex; align-items: flex-end; gap: 4px; height: 140px; }
  .wcol { flex: 1; min-width: 0; height: 100%; display: flex; flex-direction: column; justify-content: flex-end; align-items: stretch; gap: 4px; }
  .stack { display: flex; flex-direction: column-reverse; border-radius: 4px 4px 2px 2px; overflow: hidden; min-height: 2px; }
  .stack .new { background: var(--warn); }
  .stack .ret { background: var(--allies); }
  .wl { font-size: 10px; color: var(--muted); text-align: center; white-space: nowrap; overflow: visible; }
  .rows { display: flex; flex-direction: column; }
  .row { display: grid; grid-template-columns: minmax(0, 1fr) 44px 64px 72px; gap: 8px; align-items: center; min-height: 40px; padding: 0 4px; border: 0; border-top: 1px solid var(--line-soft); background: transparent; text-align: left; font-size: 14px; color: var(--text); }
  .row.gone { grid-template-columns: minmax(0, 1fr) 44px 64px 60px; }
  .row > :not(:first-child) { text-align: right; font-variant-numeric: tabular-nums; font-size: 13px; }
  .row.head { min-height: 26px; border-top: 0; font-size: 11px; font-weight: 800; color: var(--muted); text-transform: uppercase; letter-spacing: .04em; }
  .row.head > * { font-size: 11px; }
  button.row:hover { background: var(--surface-2); }
  .yes { color: var(--ok); font-weight: 700; }
  .no { color: var(--danger-text); font-weight: 700; }
  .opt { display: none; }
  @media (min-width: 600px) {
    .row:not(.gone) { grid-template-columns: minmax(0, 1fr) 130px 44px 64px 72px; }
    .opt { display: inline; }
  }
  @media (max-width: 599px) { .wcol .wl { visibility: hidden; } .wcol:nth-child(3n + 1) .wl { visibility: visible; } }
</style>
