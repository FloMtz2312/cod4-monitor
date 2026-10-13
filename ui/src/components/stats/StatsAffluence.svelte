<script>
  // Affluence: players over the period (max and average), the map changes, and when players come (day × hour).
  import Seg from '../Seg.svelte';
  import LineChart from './LineChart.svelte';
  import { get } from '../../lib/api.js';
  import { mapName } from '../../lib/state.svelte.js';
  import { href } from '../../lib/router.svelte.js';
  import { ago } from '../../lib/format.js';
  import { t, i18n } from '../../lib/i18n.svelte.js';
  import { dur, short, dec } from './util.js';

  /** @type {{ id: number, period: string, slots: number | null }} */
  let { id, period, slots } = $props();

  let hist = $state(null);
  let before = $state(null);
  let heat = $state(null);
  let weekend = $state(false);
  let shown = $state(30);
  let uptime = $state(null);

  $effect(() => {
    const sid = id, p = period;
    let alive = true;
    hist = heat = before = null;
    shown = 30;
    get(`/api/servers/${sid}/history?range=${p}`).then(r => {
      if (!alive) return;
      hist = r;
      // same length just before, to compare the average
      get(`/api/servers/${sid}/history?from=${2 * r.from - r.to}&to=${r.from}`).then(b => { if (alive) before = b; }).catch(() => {});
    }).catch(() => { if (alive) hist = {}; });
    get(`/api/servers/${sid}/uptime`).then(r => { if (alive) uptime = r; }).catch(() => {});
    get(`/api/servers/${sid}/heatmap?range=${p}`).then(r => { if (alive) heat = r; }).catch(() => { if (alive) heat = {}; });
    return () => { alive = false; };
  });

  const points = $derived(hist?.points ?? []);
  const mean = ps => {
    const v = (ps ?? []).map(x => x.avg_players).filter(x => x != null);
    return v.length ? v.reduce((a, b) => a + b, 0) / v.length : null;
  };
  const avgNow = $derived(mean(points));
  const avgBefore = $derived(before ? mean(before.points) : null);
  const diff = $derived(avgNow != null && avgBefore ? Math.round((100 * (avgNow - avgBefore)) / avgBefore) : null);
  const peak = $derived(Math.max(0, ...points.map(p => p.max_players ?? 0)));
  // y axis: peak of the period + 2, never above the server slots
  const ymax = $derived(Math.max(1, Math.min(slots ?? Infinity, peak + 2)));

  // map changes: strip over the period, newest first in the list
  // a map lasts until the next one, or until the last poll that saw the server online (not through an outage)
  const lastOn = $derived.by(() => {
    for (let i = points.length - 1; i >= 0; i--) if (points[i].avg_players != null) return points[i].t + (hist?.bucket ?? 0);
    return hist?.to ?? 0;
  });
  const changes = $derived.by(() => {
    const ms = hist?.maps ?? [];
    const end = Math.min(hist?.to ?? 0, Math.max(lastOn, ms[ms.length - 1]?.ts ?? 0));
    return ms.map((m, i) => ({ ...m, secs: Math.max(0, (ms[i + 1]?.ts ?? end) - m.ts) }));
  });
  const hue = s => [...String(s)].reduce((a, c) => (a * 31 + c.charCodeAt(0)) % 360, 7);
  const span = $derived(hist?.to ? hist.to - hist.from : 0);
  const strip = $derived(span ? changes.map(m => ({ ...m, w: (100 * m.secs) / span })) : []);
  const stripStart = $derived(span && changes[0] ? (100 * Math.max(0, changes[0].ts - hist.from)) / span : 0);
  const recent = $derived(changes.slice().reverse());

  const DAYS = $derived(i18n.lang === 'en' ? ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'] : ['lun', 'mar', 'mer', 'jeu', 'ven', 'sam', 'dim']);
  const cells = $derived(heat?.cells ?? []);
  const heatMax = $derived(Math.max(0, ...cells.flat().map(v => v ?? 0)));
  const best = $derived.by(() => {
    /** @type {{ d: number, h: number, v: number } | null} */
    let b = null;
    cells.forEach((row, d) => row.forEach((v, h) => { if (v != null && (!b || v > b.v)) b = { d, h, v }; }));
    return b && b.v > 0 ? b : null;
  });
  // weekdays or weekend: average per hour and the busiest 3 hours
  const hours = $derived.by(() => {
    const ds = cells.slice(weekend ? 5 : 0, weekend ? 7 : 5);
    return Array.from({ length: 24 }, (_, h) => {
      const v = ds.map(d => d?.[h]).filter(x => x != null);
      return v.length ? v.reduce((a, b) => a + b, 0) / v.length : 0;
    });
  });
  const hoursMax = $derived(Math.max(0.01, ...hours));
  const days = $derived({ '7d': 7, '30d': 30, '90d': 90 }[period] ?? 30);
  const outages = $derived((uptime?.outages ?? []).filter(o => o.started_at >= Date.now() / 1000 - days * 86400));
  const busiest = $derived.by(() => {
    let b = 0;
    for (let i = 1; i <= 21; i++) if (hours[i] + hours[i + 1] + hours[i + 2] > hours[b] + hours[b + 1] + hours[b + 2]) b = i;
    return b;
  });
</script>

<div class="s-grid">
  <section class="card s-box s-wide" aria-label={t('Nombre de joueurs')}>
    <div class="s-hd"><h2 class="label">{t('Nombre de joueurs')}</h2>{#if hist?.source === 'udp'}<span class="muted s-small">{t('(sondages)')}</span>{/if}</div>
    {#if hist === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else if !points.some(p => p.max_players != null)}
      <p class="muted">{t('Pas encore de données.')}</p>
    {:else}
      <div class="s-tiles">
        <div><b>{dec(avgNow)}</b><span>{t('joueurs en moyenne')}</span></div>
        <div><b>{peak}</b><span>{t('au pic')}</span></div>
        {#if diff != null}<div><b style:color={diff > 0 ? 'var(--ok)' : diff < 0 ? 'var(--danger-text)' : 'inherit'}>{diff > 0 ? '+' : ''}{diff} %</b><span>{t('vs période d\'avant')}</span></div>{/if}
      </div>
      <LineChart label={t('Joueurs sur la période')} times={points.map(p => p.t)} {ymax} height={220}
        series={[
          { label: t('Joueurs (max)'), color: 'var(--accent)', values: points.map(p => p.max_players), fill: true },
          { label: t('Joueurs (moy.)'), color: 'var(--text-2)', values: points.map(p => p.avg_players) }
        ]} />
    {/if}
    {#if strip.length}
      <div class="s-sub">
        <h3 class="label">{t('Changements de map')}</h3>
        <div class="strip" role="img" aria-label={t('Maps jouées sur la période')}>
          <span style:flex-basis="{stripStart}%" class="gap"></span>
          {#each strip as m (m.ts)}<span style:flex-basis="{m.w}%" style:background={`hsl(${hue(m.map)} 55% 55%)`} title={`${mapName(m.map)}${m.gametype ? ` (${m.gametype})` : ''} · ${short(m.ts)} · ${dur(m.secs)}`}></span>{/each}
        </div>
        <div class="changes">
          {#each recent.slice(0, shown) as m (m.ts)}
            <div class="ch">
              <span class="sw" style:background={`hsl(${hue(m.map)} 55% 55%)`}></span>
              <span class="muted when">{short(m.ts)}</span>
              <b>{mapName(m.map)}</b>
              <span class="muted gt">{m.gametype ?? ''}</span>
              <span class="muted d">{dur(m.secs)}</span>
            </div>
          {/each}
          {#if recent.length > shown}<button class="btn s-more" onclick={() => (shown += 50)}>{t('Afficher plus')}</button>{/if}
        </div>
      </div>
    {/if}
  </section>

  <section class="card s-box" aria-label={t('Activité moyenne par heure')}>
    <div class="s-hd"><h2 class="label">{t('Activité moyenne par heure')}</h2><span class="muted s-small">{heat?.source === 'enterleave' ? t('(connexions réelles)') : t('(sondages)')}</span></div>
    {#if heat === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else if !heatMax}
      <p class="muted">{t('Pas encore de données.')}</p>
    {:else}
      {#if best}<p class="big">{t('Meilleur créneau : {d} vers {h} h', { d: DAYS[best.d], h: best.h })} <span class="muted">· {best.v.toFixed(1)}</span></p>{/if}
      <div class="heat" role="img" aria-label={t('Joueurs en moyenne par jour et par heure')}>
        {#each cells as row, d (d)}
          <span class="dl">{DAYS[d]}</span>
          {#each row as v, h (h)}<span class="cell" title={`${DAYS[d]} ${h} h · ${v == null ? t('pas de données') : v.toFixed(1)}`} style:opacity={v ? 0.15 + (0.85 * v) / heatMax : 1} class:zero={!v}></span>{/each}
        {/each}
        <span></span>{#each [0, 6, 12, 18] as h (h)}<span class="hl" style:grid-column={`${h + 2} / span 6`}>{h} h</span>{/each}
      </div>
    {/if}
  </section>

  <section class="card s-box" aria-label={t('Heures de pointe')}>
    <div class="s-hd"><h2 class="label">{t('Heures de pointe')}</h2>
      <div class="seg"><Seg label={t('Jours')} value={weekend ? 'we' : 'wd'} onchange={v => (weekend = v === 'we')} items={[['wd', t('Semaine')], ['we', t('Week-end')]]} /></div>
    </div>
    {#if heat === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else if hoursMax <= 0.01}
      <p class="muted">{t('Pas assez de données')}</p>
    {:else}
      <div class="hbars" role="img" aria-label={t('Joueurs en moyenne par heure')}>
        {#each hours as v, h (h)}<span class:top={h >= busiest && h < busiest + 3} style:height="{Math.max(3, (100 * v) / hoursMax)}%" title={`${h} h : ${v.toFixed(1)}`}></span>{/each}
      </div>
      <div class="axis" aria-hidden="true"><span>0 h</span><span>6 h</span><span>12 h</span><span>18 h</span><span>23 h</span></div>
      <p class="s-small">{t('Le plus de monde entre {a} h et {b} h ({v} joueurs en moyenne)', { a: busiest, b: busiest + 3, v: dec((hours[busiest] + hours[busiest + 1] + hours[busiest + 2]) / 3) })}</p>
    {/if}
  </section>

  {#if uptime}
    <section class="card s-box" aria-label={t('Disponibilité')}>
      <div class="s-hd"><h2 class="label">{t('Disponibilité')}</h2><a class="s-small" href={href('etat')}>{t('État du serveur')} ›</a></div>
      <div class="s-tiles">
        <div><b>{uptime['7d'] == null ? '—' : `${uptime['7d'].toFixed(1)} %`}</b><span>{t('en ligne · 7 j')}</span></div>
        <div><b>{uptime['30d'] == null ? '—' : `${uptime['30d'].toFixed(1)} %`}</b><span>{t('en ligne · 30 j')}</span></div>
        <div><b>{outages.length}</b><span>{t('coupures')}</span></div>
      </div>
      {#if outages[0]}<p class="muted s-small">{t('Dernière coupure il y a {d}', { d: ago(outages[0].started_at) })}{#if outages[0].ended_at == null}{' · '}{t('en cours')}{/if}</p>{/if}
    </section>
  {/if}
</div>

<style>
  .big { font-family: var(--font-display); font-size: 18px; font-weight: 700; }
  .big .muted { font-family: var(--font); font-size: 14px; font-weight: 600; }
  .heat { display: grid; grid-template-columns: 30px repeat(24, minmax(0, 1fr)); gap: 2px; align-items: center; }
  .dl { font-size: 11px; color: var(--muted); }
  .cell { aspect-ratio: 1; border-radius: 3px; background: var(--accent); min-height: 8px; }
  .cell.zero { background: var(--surface-2); }
  .hl { font-size: 10px; color: var(--muted); }
  .strip { display: flex; margin-left: 26px; height: 18px; border-radius: 6px; overflow: hidden; background: var(--surface-2); }
  .strip span { flex-grow: 0; flex-shrink: 1; min-width: 0; }
  .strip .gap { background: transparent; }
  .changes { display: flex; flex-direction: column; max-height: 260px; overflow: auto; }
  .ch { display: grid; grid-template-columns: 10px 92px minmax(0, 1fr) auto 64px; gap: 8px; align-items: center; min-height: 32px; border-top: 1px solid var(--line-soft); font-size: 13px; }
  .ch:first-child { border-top: 0; }
  .ch b { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .sw { width: 10px; height: 10px; border-radius: 3px; }
  .when, .d { font-variant-numeric: tabular-nums; font-size: 12px; }
  .d { text-align: right; }
  .gt { font-size: 12px; }
  .seg { width: 190px; }
  .hbars { display: flex; align-items: flex-end; gap: 3px; height: 90px; }
  .hbars span { flex: 1; border-radius: 3px 3px 1px 1px; background: #33405A; }
  .hbars span.top { background: var(--accent); }
  .axis { display: flex; justify-content: space-between; font-size: 11px; color: var(--muted); }
</style>
