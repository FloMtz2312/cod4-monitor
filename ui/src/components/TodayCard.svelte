<script>
  // Players per hour over the last 24 hours (the busiest moment of each hour).
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ history: any, playersNow: number }} */
  let { history, playersNow } = $props();

  const bars = $derived.by(() => {
    const pts = history?.points ?? [];
    if (!pts.length) return [];
    const byHour = new Map();
    for (const p of pts) {
      const h = Math.floor(p.t / 3600);
      byHour.set(h, Math.max(byHour.get(h) ?? 0, p.max_players ?? 0));
    }
    const nowH = Math.floor(Date.now() / 3600000);
    const out = [];
    for (let h = nowH - 23; h <= nowH; h++) out.push({ h, v: byHour.get(h) ?? 0, now: h === nowH });
    return out;
  });
  const peak = $derived(bars.reduce((a, b) => (b.v > a.v ? b : a), { v: 0, h: 0 }));
  const max = $derived(Math.max(1, peak.v));
  const hourLabel = h => `${new Date(h * 3600000).getHours()} h`;
</script>

{#if bars.length && (peak.v > 0 || playersNow > 0)}
  <section class="card today" aria-label={t('Dernières 24 heures')}>
    <div class="head"><h2 class="label">{t('Dernières 24 heures')}</h2><span class="muted">{t('joueurs par heure')}</span></div>
    <div class="bars" aria-hidden="true">
      {#each bars as b (b.h)}<span class:now={b.now} class:empty={!b.v} style:height="{Math.max(4, (b.v / max) * 56)}px"></span>{/each}
    </div>
    <div class="axis" aria-hidden="true"><span>{hourLabel(bars[0].h)}</span><span>{hourLabel(bars[12].h)}</span><span>{t('maintenant')}</span></div>
    <div class="stats">
      <span><b>{playersNow}</b> {t('en ligne')}</span>
      <span><b>{peak.v}</b> {peak.v ? t('au pic, vers {h}', { h: hourLabel(peak.h) }) : t('au pic')}</span>
    </div>
  </section>
{/if}

<style>
  .today { padding: 14px 16px; }
  .head { display: flex; justify-content: space-between; align-items: baseline; }
  .head .muted { font-size: 12px; }
  .bars { display: flex; align-items: flex-end; gap: 3px; height: 56px; margin: 12px 0 4px; }
  .bars span { flex: 1; border-radius: 3px; background: #33405A; }
  .bars span.empty { background: var(--line-soft); }
  .bars span.now { background: var(--accent); }
  .axis { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); }
  .stats { display: flex; gap: 16px; margin-top: 10px; font-size: 13px; color: var(--muted); }
  .stats b { color: var(--text); font-family: var(--font-display); font-size: 18px; }
</style>
