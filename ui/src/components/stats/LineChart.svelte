<script>
  // Line chart over time without a chart library: an SVG stretched to the width, HTML axis labels,
  // and a readout of the values under the finger / pointer. A null value leaves a gap.
  import { short, dayLabel } from './util.js';

  /** @typedef {{ label: string, color: string, values: (number | null)[], fill?: boolean }} Series */
  /** @type {{ times: number[], series: Series[], label: string, ymax?: number, height?: number, unit?: string, digits?: number }} */
  let { times, series, label, ymax = 0, height = 200, unit = '', digits = 1 } = $props();

  const H = 100;
  const n = $derived(times.length);
  const top = $derived.by(() => {
    const m = ymax || Math.max(0, ...series.flatMap(s => s.values.filter(v => v != null)));
    return m > 0 ? m : 1;
  });
  // about 4 grid lines on round values
  const step = $derived([1, 2, 5, 10, 20, 25, 50, 100, 200, 250, 500, 1000].find(s => top / s <= 4) ?? Math.ceil(top / 4));
  const ticks = $derived(Array.from({ length: Math.floor(top / step) + 1 }, (_, i) => i * step));
  const x = i => (n > 1 ? (1000 * i) / (n - 1) : 500);
  const y = v => H - (H * Math.min(v, top)) / top;

  /** Line segments of a series, split at the missing values. */
  function paths(values) {
    const segs = [];
    let cur = [];
    values.forEach((v, i) => {
      if (v == null) { if (cur.length) segs.push(cur); cur = []; }
      else cur.push([x(i), y(v)]);
    });
    if (cur.length) segs.push(cur);
    return segs.map(s => ({
      line: 'M' + s.map(([a, b]) => `${a.toFixed(1)},${b.toFixed(2)}`).join('L') + (s.length === 1 ? 'h2' : ''),
      area: `M${s[0][0].toFixed(1)},${H}L` + s.map(([a, b]) => `${a.toFixed(1)},${b.toFixed(2)}`).join('L') + `L${s[s.length - 1][0].toFixed(1)},${H}Z`
    }));
  }
  const drawn = $derived(series.map(s => ({ ...s, segs: paths(s.values) })));
  const spanS = $derived(n > 1 ? times[n - 1] - times[0] : 0);
  const xLabels = $derived(n > 1 ? [0, 0.25, 0.5, 0.75, 1].map(f => {
    const i = Math.round(f * (n - 1));
    const ts = times[i];
    return { f, text: spanS <= 2 * 86400 ? new Date(ts * 1000).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : dayLabel(ts) };
  }) : []);

  let hover = $state(-1);
  /** @param {PointerEvent} e */
  function move(e) {
    const r = /** @type {HTMLElement} */ (e.currentTarget).getBoundingClientRect();
    hover = n ? Math.max(0, Math.min(n - 1, Math.round(((e.clientX - r.left) / r.width) * (n - 1)))) : -1;
  }
  const fmtV = v => (v == null ? '—' : `${Number(v).toFixed(digits).replace(/\.0+$/, '')}${unit}`);
</script>

<figure class="lc">
  <div class="plot" style:height="{height}px" onpointermove={move} onpointerleave={() => (hover = -1)} role="presentation">
    {#each ticks as v (v)}<span class="yl" style:bottom="{(100 * v) / top}%">{v}</span>{/each}
    <svg viewBox="0 0 1000 {H}" preserveAspectRatio="none" role="img" aria-label={label}>
      <title>{label}</title>
      {#each ticks as v (v)}<line x1="0" x2="1000" y1={y(v)} y2={y(v)} class="grid" vector-effect="non-scaling-stroke" />{/each}
      {#each drawn as s (s.label)}
        {#each s.segs as p, i (i)}
          {#if s.fill}<path d={p.area} fill={s.color} opacity=".14" />{/if}
          <path d={p.line} fill="none" stroke={s.color} stroke-width="2" vector-effect="non-scaling-stroke" stroke-linejoin="round" />
        {/each}
      {/each}
    </svg>
    {#if hover >= 0}
      <span class="cursor" style:left="{(100 * x(hover)) / 1000}%"></span>
      <div class="tip" class:right={hover > n / 2} style:left="{(100 * x(hover)) / 1000}%">
        <b>{short(times[hover])}</b>
        {#each series as s (s.label)}<span><i style:background={s.color}></i>{s.label} : {fmtV(s.values[hover])}</span>{/each}
      </div>
    {/if}
  </div>
  <div class="xl" aria-hidden="true">{#each xLabels as l (l.f)}<span>{l.text}</span>{/each}</div>
  {#if series.length > 1}
    <figcaption>{#each series as s (s.label)}<span><i style:background={s.color}></i>{s.label}</span>{/each}</figcaption>
  {/if}
</figure>

<style>
  .lc { margin: 0; display: flex; flex-direction: column; gap: 4px; min-width: 0; }
  .plot { position: relative; margin-left: 26px; touch-action: pan-y; }
  svg { position: absolute; inset: 0; width: 100%; height: 100%; overflow: visible; }
  .grid { stroke: var(--line-soft); stroke-width: 1; }
  .yl { position: absolute; left: -26px; width: 20px; text-align: right; transform: translateY(50%); font-size: 10px; color: var(--muted); font-variant-numeric: tabular-nums; }
  .xl { display: flex; justify-content: space-between; margin-left: 26px; font-size: 10px; color: var(--muted); }
  .cursor { position: absolute; top: 0; bottom: 0; width: 1px; background: var(--text-2); opacity: .5; pointer-events: none; }
  .tip { position: absolute; top: 0; transform: translateX(8px); display: flex; flex-direction: column; gap: 2px; padding: 6px 8px; border-radius: 10px; background: var(--surface-2); border: 1px solid var(--line); font-size: 12px; white-space: nowrap; pointer-events: none; z-index: 1; }
  .tip.right { transform: translateX(calc(-100% - 8px)); }
  i { display: inline-block; width: 8px; height: 8px; border-radius: 2px; margin-right: 5px; }
  figcaption { display: flex; gap: 14px; flex-wrap: wrap; font-size: 12px; color: var(--muted); margin-left: 26px; }
</style>
