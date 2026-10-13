<script>
  // The last 7 days of the server: player of the week, top 5, records. Shown on the live screen, also when nobody plays.
  import CodName from './CodName.svelte';
  import { mapName } from '../lib/state.svelte.js';
  import { fmt } from '../lib/format.js';
  import { dt } from '../lib/dates.js';
  import { href } from '../lib/router.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ week: any }} */
  let { week } = $props();
  const top = $derived(week?.top ?? []);
  const star = $derived(top[0]);
  const kd = p => (p.kills / Math.max(1, p.deaths)).toFixed(2);
  const hours = $derived(Math.round((week?.play_secs ?? 0) / 3600));
  const records = $derived([
    week?.longest && { label: t('Plus long kill'), value: `${Math.round(week.longest.value)} m`, p: week.longest },
    week?.headshots && { label: t('Headshots'), value: fmt(week.headshots.value), p: week.headshots },
    week?.noscopes && { label: t('Noscopes'), value: fmt(week.noscopes.value), p: week.noscopes }
  ].filter(Boolean));
  const link = key => href('joueur', key);
</script>

{#if star}
  <section class="card week" aria-label={t('Cette semaine')}>
    <div class="head"><h2 class="label">{t('Cette semaine')}</h2><span class="muted">{t('7 derniers jours')}</span></div>

    <a class="star" href={link(star.key)}>
      <span class="badge">{t('Joueur de la semaine')}</span>
      <span class="sname"><CodName raw={star.name_raw} /></span>
      <span class="sline">{fmt(star.kills)} kills · K/D {kd(star)} · {fmt(star.headshots)} headshots</span>
    </a>

    {#if top.length > 1}
      <ol class="top">
        {#each top as p, i (p.key)}
          <li><a href={link(p.key)}>
            <span class="rank" class:podium={i < 3}>{i + 1}</span>
            <span class="pn"><CodName raw={p.name_raw} /></span>
            <span class="pk">{fmt(p.kills)}</span>
            <span class="pd">K/D {kd(p)}</span>
          </a></li>
        {/each}
      </ol>
    {/if}

    {#if records.length}
      <div class="records">
        {#each records as r (r.label)}
          <a class="rec" href={link(r.p.key)}>
            <span class="rl">{r.label}</span>
            <span class="rv">{r.value}</span>
            <span class="rp"><CodName raw={r.p.name_raw} /></span>
          </a>
        {/each}
      </div>
    {/if}

    <div class="stats">
      <span><b>{fmt(week.players)}</b> {t('joueurs')}</span>
      <span><b>{fmt(week.matches)}</b> {t('matchs')}</span>
      <span><b>{hours} h</b> {t('de jeu')}</span>
      {#if week.kills}<span><b>{fmt(week.kills)}</b> {t('kills')}</span>{/if}
      {#if week.peak}<span><b>{fmt(week.peak)}</b> {t('joueurs au pic')}{#if week.peak_at}{' · '}{dt(week.peak_at)}{/if}</span>{/if}
      {#if week.map}<span><b>{mapName(week.map)}</b> {t('map préférée')}</span>{/if}
    </div>
  </section>
{/if}

<style>
  .week { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; }
  .head { display: flex; justify-content: space-between; align-items: baseline; }
  .head .muted { font-size: 12px; }
  a { color: inherit; text-decoration: none; }
  .star { display: flex; flex-direction: column; gap: 2px; padding: 14px; border-radius: 16px; border: 1px solid var(--accent); background: var(--accent-soft); }
  .badge { font-size: 12px; font-weight: 800; color: var(--accent); letter-spacing: .04em; text-transform: uppercase; }
  .sname { font-family: var(--font-display); font-size: 24px; font-weight: 700; display: flex; min-width: 0; }
  .sline { font-size: 13px; color: var(--text-2); }
  .top { list-style: none; margin: 0; padding: 0; }
  .top a { display: flex; align-items: center; gap: 10px; height: 44px; border-top: 1px solid var(--line-soft); }
  .top li:first-child a { border-top: 0; }
  .rank { width: 26px; height: 26px; flex: none; border-radius: 8px; display: flex; align-items: center; justify-content: center; font-size: 12px; font-weight: 800; background: var(--surface-2); color: var(--text-2); }
  .rank.podium { background: var(--accent); color: var(--on-accent); }
  .pn { flex: 1; min-width: 0; display: flex; font-weight: 700; }
  .pk { font-family: var(--font-display); font-weight: 700; }
  .pd { width: 64px; text-align: right; font-size: 12px; color: var(--muted); }
  .records { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .rec { display: flex; flex-direction: column; gap: 2px; padding: 10px; border-radius: 14px; background: var(--surface-2); min-width: 0; }
  .rl { font-size: 11px; color: var(--muted); font-weight: 700; }
  .rv { font-family: var(--font-display); font-size: 19px; font-weight: 700; }
  .rp { font-size: 12px; display: flex; min-width: 0; }
  .stats { display: flex; flex-wrap: wrap; gap: 6px 16px; font-size: 13px; color: var(--muted); }
  .stats b { color: var(--text); }
</style>
