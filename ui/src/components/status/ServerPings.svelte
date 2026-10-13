<script>
  // Ping of the players over the period (status polls): summary, average by hour, and per player.
  import CodName from '../CodName.svelte';
  import FilterInput from '../stats/FilterInput.svelte';
  import { go } from '../../lib/router.svelte.js';
  import { clean } from '../../lib/format.js';
  import { t } from '../../lib/i18n.svelte.js';
  import { dur, playerKey } from '../stats/util.js';

  /** @type {{ data: any, interval: number }} */
  let { data, interval } = $props();

  let q = $state('');
  let shown = $state(30);
  const high = $derived(data?.high ?? 150);
  const players = $derived(data?.players ?? []);
  const hours = $derived(data?.hours ?? []);
  const hv = $derived(hours.filter(v => v != null));
  const hmax = $derived(Math.max(1, ...hv));
  const worst = $derived(hv.length ? hours.indexOf(Math.max(...hv)) : -1);
  const list = $derived.by(() => {
    const f = q.trim().toLowerCase();
    return players.filter(x => !f || clean(x.name_raw ?? x.name_clean).toLowerCase().includes(f));
  });
  const ms = v => (v == null ? '—' : `${Math.round(v)} ms`);
  const col = v => (v == null ? 'var(--muted)' : v >= 150 ? 'var(--danger)' : v >= 90 ? 'var(--warn)' : 'var(--ok)');
  const textCol = v => (v == null ? 'inherit' : v >= 150 ? 'var(--danger-text)' : v >= 90 ? 'var(--warn)' : 'inherit');
</script>

<section class="card s-box s-wide" aria-label={t('Ping des joueurs')}>
  <div class="s-hd"><h2 class="label">{t('Ping des joueurs')}</h2><span class="muted s-small">{t('au-delà de {n} ms, un duel au sniper est déjà mal parti', { n: high })}</span></div>
  {#if !data}
    <p class="muted">{t('Chargement…')}</p>
  {:else}
    <div class="s-tiles">
      <div><b style:color={col(data.avg)}>{ms(data.avg)}</b><span>{t('ping moyen des joueurs')}</span></div>
      <div><b>{players.filter(x => x.avg >= high).length}</b><span>{t('joueurs à plus de {n} ms en moyenne', { n: high })}</span></div>
      <div><b>{worst < 0 ? '—' : `${worst} h`}</b><span>{t('heure au ping le plus haut')}</span></div>
    </div>
    <div class="s-sub">
      <h3 class="label">{t('Ping moyen selon l\'heure')}</h3>
      <div class="hbars" role="img" aria-label={t('Ping moyen selon l\'heure')}>
        {#each hours as v, h (h)}<span title={`${h} h : ${ms(v)}`} style:height="{v == null ? 0 : Math.max(3, (100 * v) / hmax)}%" style:background={col(v)}></span>{/each}
      </div>
      <div class="axis" aria-hidden="true"><span>0 h</span><span>6 h</span><span>12 h</span><span>18 h</span><span>23 h</span></div>
    </div>
    <div class="s-sub">
      <div class="s-hd"><h3 class="label">{t('Par joueur')}</h3><FilterInput bind:value={q} label={t('Filtrer…')} /></div>
      {#if list.length}
        <div class="rows">
          <div class="row head" aria-hidden="true"><span>{t('Joueur')}</span><span>{t('Moyen')}</span><span>{t('Max')}</span><span title={t('part des relevés au-dessus de {n} ms', { n: high })}>&gt; {high} ms</span><span class="opt">{t('Temps relevé')}</span></div>
          {#each list.slice(0, shown) as x (x.name_clean)}
            <button class="row" onclick={() => go('joueur', playerKey(x.guid, x.name_clean))}>
              <CodName raw={x.name_raw ?? x.name_clean} />
              <b style:color={textCol(x.avg)}>{ms(x.avg)}</b>
              <span class="muted">{ms(x.max)}</span>
              <span>{Math.round(100 * (x.high ?? 0))} %</span>
              <span class="muted opt">{dur(x.samples * interval)}</span>
            </button>
          {/each}
        </div>
        {#if list.length > shown}<button class="btn s-more" onclick={() => (shown += 50)}>{t('Afficher plus')}</button>{/if}
      {:else}
        <p class="muted">{t('Aucun relevé sur la période')}</p>
      {/if}
    </div>
  {/if}
</section>

<style>
  .hbars { display: flex; align-items: flex-end; gap: 3px; height: 80px; }
  .hbars span { flex: 1; border-radius: 3px 3px 1px 1px; }
  .axis { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); }
  .rows { display: flex; flex-direction: column; }
  .row { display: grid; grid-template-columns: minmax(0, 1fr) 64px 58px 64px; gap: 8px; align-items: center; min-height: 40px; padding: 0 4px; border: 0; border-top: 1px solid var(--line-soft); background: transparent; text-align: left; font-size: 14px; color: var(--text); }
  .row > :not(:first-child) { text-align: right; font-variant-numeric: tabular-nums; }
  .row.head { min-height: 26px; border-top: 0; font-size: 11px; font-weight: 800; color: var(--muted); text-transform: uppercase; letter-spacing: .04em; }
  button.row:hover { background: var(--surface-2); }
  .opt { display: none; }
  @media (min-width: 600px) {
    .row { grid-template-columns: minmax(0, 1fr) 80px 70px 80px 100px; }
    .opt { display: inline; }
  }
</style>
