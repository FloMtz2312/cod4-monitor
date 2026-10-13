<script>
  // Résumé of a profile: form over the last matches, recent form (7 days vs the 7 before, comparison), when they play,
  // weekly progression (K/D, headshots or accuracy).
  import CodName from '../CodName.svelte';
  import PlayerForm from './PlayerForm.svelte';
  import { mapName } from '../../lib/state.svelte.js';
  import { t } from '../../lib/i18n.svelte.js';

  /** @type {{ p: any, form: any, matches: any[] | null, progress: any[] | null, playerKey: string, serverId: number }} */
  let { p, form, matches, progress, playerKey, serverId } = $props();

  const kd = (k, d) => (k ?? 0) / Math.max(1, d ?? 0);

  // the last 10 matches, oldest on the left; bar height = K/D
  const recent = $derived((matches ?? []).slice(0, 10).reverse().map(m => ({
    kd: kd(m.player?.kills, m.player?.deaths), result: m.player?.result ?? '',
    title: `${mapName(m.match.map)} · ${m.player?.kills ?? 0}/${m.player?.deaths ?? 0}`
  })));
  const formTop = $derived(Math.max(2, ...recent.map(r => r.kd)));

  const hours = $derived(p.hours ?? []);
  const hoursMax = $derived(Math.max(0, ...hours));
  const peak = $derived(hours.indexOf(hoursMax));

  // weekly value of the chosen metric; weeks without data for it are left out
  const METRICS = [['kd', 'K/D'], ['hs', 'Headshots'], ['acc', 'Précision']];
  let metric = $state('kd');
  const valueOf = (w, m) => m === 'kd' ? ((w.kills ?? 0) + (w.deaths ?? 0) > 0 ? kd(w.kills, w.deaths) : null)
    : m === 'hs' ? (w.kills ? (100 * (w.headshots ?? 0)) / w.kills : null)
    : (w.shots ? (100 * (w.hits ?? 0)) / w.shots : null);
  const show = (v, m) => (m === 'kd' ? v.toFixed(2) : `${Math.round(v)} %`);
  const weeks = $derived((progress ?? []).map(w => ({ w, v: valueOf(w, metric) })).filter(x => x.v != null).slice(-12)
    .map(({ w, v }) => ({ v, title: `${w.week} · ${show(v, metric)}` })));
  const weeksTop = $derived(metric === 'kd' ? Math.max(2, ...weeks.map(w => w.v)) : Math.max(10, ...weeks.map(w => w.v)));
  const anyWeeks = $derived((progress ?? []).some(w => (w.kills ?? 0) + (w.deaths ?? 0) > 0));

  const cur = $derived(form?.current);
  // the other names are in the header for the staff
  const others = $derived(p.sessions === undefined ? (p.names ?? []).slice(1, 8) : []);
</script>

<div class="stack">
  {#if recent.length}
    <section class="card box" aria-label={t('Forme récente')}>
      <div class="hd"><h2 class="label">{t('Forme · matchs récents')}</h2><span class="muted">K/D</span></div>
      <div class="bars form" aria-hidden="true">
        {#each recent as r, i (i)}
          <span title={r.title} class={r.result} style:height={`${Math.max(8, (r.kd / formTop) * 100)}%`}></span>
        {/each}
      </div>
      <ul class="sr-only">{#each recent as r, i (i)}<li>{r.title} · {r.result === 'win' ? t('victoire') : r.result === 'loss' ? t('défaite') : ''}</li>{/each}</ul>
      <div class="legend"><span><i class="win"></i>{t('victoire')}</span><span><i class="loss"></i>{t('défaite')}</span><span><i></i>{t('sans résultat')}</span></div>
    </section>
  {/if}

  {#if cur && (cur.kills + cur.deaths > 0 || form.previous.kills + form.previous.deaths > 0 || form.recent?.length)}
    <PlayerForm {form} {playerKey} {serverId} />
  {/if}

  {#if hoursMax > 0}
    <section class="card box" aria-label={t('Heures de jeu')}>
      <h2 class="label">{t('Joue surtout')}</h2>
      <p class="big">{t('Vers {h} h', { h: peak })}</p>
      <div class="bars hours" aria-hidden="true">
        {#each hours as h, i (i)}<span class:peak={i === peak} style:height={`${Math.max(6, (h / hoursMax) * 100)}%`}></span>{/each}
      </div>
      <div class="axis" aria-hidden="true"><span>0 h</span><span>6 h</span><span>12 h</span><span>18 h</span><span>23 h</span></div>
    </section>
  {/if}

  {#if anyWeeks}
    <section class="card box" aria-label={t('Progression')}>
      <div class="hd"><h2 class="label">{t('Progression · {n} semaines', { n: weeks.length })}</h2>
        {#if weeks.length}<span class="now">{show(weeks[weeks.length - 1].v, metric)}</span>{/if}</div>
      <div class="metrics" role="radiogroup" aria-label={t('Indicateur')}>
        {#each METRICS as [k, label] (k)}
          <button role="radio" aria-checked={metric === k} class:on={metric === k} onclick={() => (metric = k)}>{t(label)}</button>
        {/each}
      </div>
      {#if weeks.length > 1}
        <div class="bars weeks" aria-hidden="true">
          {#each weeks as w, i (i)}<span title={w.title} class:last={i === weeks.length - 1} style:height={`${Math.max(6, (w.v / weeksTop) * 100)}%`}></span>{/each}
        </div>
        <ul class="sr-only">{#each weeks as w, i (i)}<li>{w.title}</li>{/each}</ul>
        <div class="axis" aria-hidden="true"><span>{t('il y a {n} sem.', { n: weeks.length - 1 })}</span><span>{t('cette semaine')}</span></div>
      {:else}
        <p class="muted small">{t('Pas assez de données')}</p>
      {/if}
    </section>
  {/if}

  {#if others.length}
    <section class="card box" aria-label={t('Aussi connu sous')}>
      <h2 class="label">{t('Aussi connu sous')}</h2>
      <div class="names">{#each others as n (n.name_clean)}<span><CodName raw={n.name_raw} /></span>{/each}</div>
    </section>
  {/if}

  {#if !recent.length && !(cur && cur.kills + cur.deaths) && !hoursMax && !anyWeeks}
    <section class="card box"><p class="muted">{t('Pas encore assez de parties enregistrées pour ce joueur.')}</p></section>
  {/if}
</div>

<style>
  .stack { display: flex; flex-direction: column; gap: 12px; }
  .box { padding: 14px 16px; }
  .hd { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; }
  .hd .muted { font-size: 12px; }
  .now { font-family: var(--font-display); font-weight: 700; color: var(--accent); }
  .bars { display: flex; align-items: flex-end; gap: 4px; height: 72px; margin-top: 12px; }
  .bars span { flex: 1; min-width: 0; border-radius: 4px 4px 2px 2px; background: #33405A; }
  /* fixed slots (10 matches, 12 weeks), the newest on the right: a short history doesn't make giant bars */
  .bars.form { justify-content: flex-end; }
  .bars.form span { flex: 0 0 calc((100% - 36px) / 10); }
  .bars.weeks { justify-content: flex-end; }
  .bars.weeks span { flex: 0 0 calc((100% - 44px) / 12); }
  .form span.win { background: var(--accent); }
  .form span.loss { background: #4A5568; }
  .hours { height: 48px; gap: 2px; }
  .hours span.peak, .weeks span.last { background: var(--accent); }
  .legend { display: flex; gap: 14px; margin-top: 10px; font-size: 12px; color: var(--muted); flex-wrap: wrap; }
  .legend span { display: inline-flex; align-items: center; gap: 6px; }
  .legend i { width: 10px; height: 10px; border-radius: 3px; background: #33405A; }
  .legend i.win { background: var(--accent); }
  .legend i.loss { background: #4A5568; }
  .axis { display: flex; justify-content: space-between; margin-top: 6px; font-size: 11px; color: var(--muted); }
  .big { margin: 4px 0 0; font-family: var(--font-display); font-size: 20px; font-weight: 700; }
  .metrics { display: flex; gap: 6px; margin-top: 10px; }
  .metrics button { height: 32px; padding: 0 12px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); font-weight: 700; font-size: 13px; }
  .metrics button.on { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .names { display: flex; flex-wrap: wrap; gap: 6px; margin-top: 10px; }
  .names span { display: inline-flex; height: 30px; align-items: center; padding: 0 10px; border-radius: 10px; background: var(--surface-2); font-size: 14px; font-weight: 700; max-width: 100%; }
  .small { font-size: 12px; margin-top: 10px; }
  p { margin: 0; }
  @media (min-width: 960px) {
    .stack { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; }
  }
</style>
