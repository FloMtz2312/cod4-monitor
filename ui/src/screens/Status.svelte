<script>
  // État du serveur: key figures, availability and outages, server details, latency, players' ping,
  // console errors and variables (private), log collection and the monitor's machine (account admins).
  import BackTitle from '../components/BackTitle.svelte';
  import Seg from '../components/Seg.svelte';
  import CodName from '../components/CodName.svelte';
  import LineChart from '../components/stats/LineChart.svelte';
  import FilterInput from '../components/stats/FilterInput.svelte';
  import ServerPings from '../components/status/ServerPings.svelte';
  import LogHealth from '../components/status/LogHealth.svelte';
  import '../components/stats/stats.css';
  import { get } from '../lib/api.js';
  import { app, currentServer, mapName } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { agoText, clean } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';
  import { dt } from '../components/stats/util.js';

  const srv = $derived(currentServer());
  const id = $derived(app.serverId);
  const admin = $derived(['admin', 'superadmin'].includes(app.me?.role) || !!app.me?.noauth);
  const priv = $derived(permsOf(app.me, id).has('private'));
  let period = $state('24h');
  let up = $state(null);
  let vps = $state(null);
  let status = $state(null);
  let hist = $state(null);
  let pings = $state(null);
  let errors = $state(null);
  let interval = $state(15);
  let varQ = $state('');

  $effect(() => {
    const sid = id;
    if (!sid) return;
    let alive = true;
    up = status = errors = null;
    get(`/api/servers/${sid}/uptime`).then(r => { if (alive) up = r; }).catch(() => { if (alive) up = {}; });
    get(`/api/servers/${sid}/status`).then(r => { if (alive) status = r; }).catch(() => { if (alive) status = {}; });
    if (priv) get(`/api/servers/${sid}/errors`).then(r => { if (alive) errors = r.errors ?? []; }).catch(() => { if (alive) errors = []; });
    return () => { alive = false; };
  });
  $effect(() => {
    const sid = id, p = period;
    if (!sid) return;
    let alive = true;
    hist = pings = null;
    get(`/api/servers/${sid}/history?range=${p}`).then(r => { if (alive) hist = r; }).catch(() => { if (alive) hist = {}; });
    if (priv) get(`/api/servers/${sid}/pings?range=${p}`).then(r => { if (alive) pings = r; }).catch(() => { if (alive) pings = {}; });
    return () => { alive = false; };
  });
  $effect(() => { if (admin) get('/api/vps').then(r => { vps = r; }).catch(() => {}); });
  $effect(() => { get('/api/servers').then(r => { interval = r.poll_interval_s || 15; }).catch(() => {}); });

  const pct = v => (v == null ? '—' : `${v >= 99.95 ? 100 : v.toFixed(1)} %`);
  const dur = s => (s < 60 ? `${s} s` : s < 3600 ? `${Math.round(s / 60)} min` : s < 86400 ? `${Math.floor(s / 3600)} h ${String(Math.round((s % 3600) / 60)).padStart(2, '0')}` : `${Math.round(s / 86400)} ${t('j')}`);
  const cause = o => o.clean ? t('arrêt propre') : o.restart ? t('redémarrage') : o.reason ? t(o.reason) : t('cause inconnue');
  const gauge = v => (v == null || v < 0 ? null : Math.round(v));
  // green under a, orange under b, red above
  const col = (x, a, b) => (x == null ? 'inherit' : x < a ? 'var(--ok)' : x < b ? 'var(--warn)' : 'var(--danger-text)');

  const days = $derived({ '24h': 1, '7d': 7, '30d': 30 }[period]);
  const periodOut = $derived((up?.outages ?? []).filter(o => o.started_at >= Date.now() / 1000 - days * 86400));
  const availability = $derived(up?.[period]);

  const snap = $derived(status?.snapshot?.[0] ?? {});
  const vars = $derived(status?.vars ?? {});
  const online = $derived(snap.online === 1);
  const points = $derived(hist?.points ?? []);
  const latency = $derived.by(() => {
    if (online && snap.latency_ms != null) return snap.latency_ms;
    for (let i = points.length - 1; i >= 0; i--) if (points[i].latency_ms != null) return points[i].latency_ms;
    return null;
  });
  const hasLatency = $derived(points.some(p => p.latency_ms != null));
  const high = $derived(pings?.high ?? 150);

  const details = $derived.by(() => {
    const lastOk = status?.last_ok?.[0]?.ts;
    return [
      [t('Statut'), online ? t('En ligne') : snap.ts ? t('Hors ligne') : '—', online ? 'var(--ok)' : snap.ts ? 'var(--danger-text)' : ''],
      [t('Map'), snap.map || vars.mapname ? `${mapName(snap.map || vars.mapname)} (${snap.map || vars.mapname})` : '—'],
      [t('Mode'), snap.gametype || vars.g_gametype || '—'],
      [t('Joueurs'), online ? `${snap.players} / ${snap.max_players}` : '—'],
      [t('Latence'), online && snap.latency_ms != null ? `${snap.latency_ms.toFixed(1)} ms` : '—'],
      [t('Dernier sondage OK'), lastOk ? `${dt(lastOk)} · ${agoText(lastOk)}` : '—'],
      [t('Map lancée'), vars.g_mapStartTime || '—'],
      [t('Mod'), vars.fs_game || '—'],
      [t('Version'), vars.version || vars.shortversion || '—']
    ];
  });
  const hostname = $derived(snap.hostname || vars.sv_hostname || '');
  const varRows = $derived.by(() => {
    const f = varQ.trim().toLowerCase();
    return Object.entries(vars).sort((a, b) => a[0].localeCompare(b[0]))
      .filter(([k, v]) => !f || k.toLowerCase().includes(f) || clean(v).toLowerCase().includes(f));
  });
</script>

<BackTitle title={t('État du serveur')} sub={srv ? srv.name.replace(/\^\d/g, '') : ''} />
<Seg label={t('Période')} value={period} onchange={v => (period = v)} items={[['24h', t('24 h')], ['7d', t('7 jours')], ['30d', t('30 jours')]]} />

{#if up === null}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <div class="s-grid">
    <section class="card s-box s-wide" aria-label={t('Chiffres clés')}>
      <div class="s-tiles kpis">
        <div>
          <span>{t('Disponibilité')}</span>
          <b style:color={availability == null ? 'inherit' : availability > 99 ? 'var(--ok)' : availability > 95 ? 'var(--warn)' : 'var(--danger-text)'}>{pct(availability)}</b>
          <span>{t('{n} coupure(s)', { n: periodOut.length })}</span>
        </div>
        <div>
          <span>{t('Latence du serveur')}</span>
          <b style:color={col(latency, 60, 120)}>{latency == null ? '—' : `${Math.round(latency)} ms`}</b>
          <span>{t('depuis le monitor')}</span>
        </div>
        {#if priv}
          <div>
            <span>{t('Ping des joueurs')}</span>
            <b style:color={col(pings?.avg, 90, 150)}>{pings?.avg == null ? '—' : `${Math.round(pings.avg)} ms`}</b>
            <span>{t('{n} joueur(s) au-dessus de {h} ms', { n: (pings?.players ?? []).filter(x => x.avg >= high).length, h: high })}</span>
          </div>
        {/if}
      </div>
    </section>

    <section class="card s-box" aria-label={t('Disponibilité')}>
      <h2 class="label">{t('Disponibilité')}</h2>
      <div class="s-tiles three">
        <div><b>{pct(up['24h'])}</b><span>24 h</span></div>
        <div><b>{pct(up['7d'])}</b><span>{t('7 jours')}</span></div>
        <div><b>{pct(up['30d'])}</b><span>{t('30 jours')}</span></div>
      </div>
      {#if up.days?.length}
        <div class="days" aria-hidden="true">
          {#each up.days as d (d.day)}<span title={`${d.day} · ${pct(d.pct)}`} class:bad={d.pct < 99} class:worse={d.pct < 90}></span>{/each}
        </div>
        <p class="muted s-small">{t('Un trait par jour (14 jours) · orange : coupure · rouge : longue coupure')}</p>
      {/if}
    </section>

    <section class="card s-box" aria-label={t('Coupures')}>
      <h2 class="label">{t('Coupures (30 jours)')}</h2>
      {#each up.outages ?? [] as o (o.started_at)}
        <details class="out">
          <summary>
            <span class="dot" class:on={o.ended_at == null}></span>
            <span class="ot"><b>{o.ended_at == null ? t('En cours depuis {d}', { d: dur(o.duration_s) }) : dur(o.duration_s)}</b>
              <span class="muted">{agoText(o.started_at)} · {cause(o)}</span></span>
          </summary>
          {#if o.console}<pre>{o.console}</pre>{:else}<p class="muted s-small">{t('Pas de détail de console.')}</p>{/if}
        </details>
      {:else}
        <p class="muted">{t('Aucune coupure ces 30 derniers jours.')}</p>
      {/each}
    </section>

    <section class="card s-box" aria-label={t('Détails du serveur')}>
      <h2 class="label">{t('Détails du serveur')}</h2>
      {#if status === null}
        <p class="muted">{t('Chargement…')}</p>
      {:else}
        <dl class="kv">
          <dt>{t('Hostname')}</dt><dd>{#if hostname}<CodName raw={hostname} />{:else}—{/if}</dd>
          {#each details as [k, v, c] (k)}<dt>{k}</dt><dd style:color={c || null}>{v}</dd>{/each}
        </dl>
      {/if}
    </section>

    {#if priv}
      <section class="card s-box" aria-label={t('Latence du serveur')}>
        <div class="s-hd"><h2 class="label">{t('Latence du serveur')}</h2><span class="muted s-small">{t('depuis le monitor')}</span></div>
        {#if hist === null}
          <p class="muted">{t('Chargement…')}</p>
        {:else if !hasLatency}
          <p class="muted">{t('Aucun relevé sur la période')}</p>
        {:else}
          <LineChart label={t('Latence du serveur (ms)')} times={points.map(p => p.t)} unit=" ms" digits={0} height={180}
            series={[{ label: t('Latence'), color: 'var(--ok)', values: points.map(p => p.latency_ms), fill: true }]} />
        {/if}
      </section>

      <ServerPings data={pings} {interval} />

      <section class="card s-box" aria-label={t('Erreurs de la console')}>
        <h2 class="label">{t('Erreurs de la console')}</h2>
        {#if errors === null}
          <p class="muted">{t('Chargement…')}</p>
        {:else if !errors.length}
          <p class="muted">{t('Aucune erreur relevée')}</p>
        {:else}
          <div class="errs">
            {#each errors as e (e.message)}
              <div class="err">
                <code>{e.message}</code>
                <span class="muted s-small">{t('{n} fois', { n: e.count })} · {agoText(e.last_seen)}</span>
              </div>
            {/each}
          </div>
        {/if}
      </section>

      <section class="card s-box" aria-label={t('Variables serveur')}>
        <div class="s-hd"><h2 class="label">{t('Variables serveur')}</h2>{#if Object.keys(vars).length}<FilterInput bind:value={varQ} label={t('Filtrer…')} />{/if}</div>
        {#if status === null}
          <p class="muted">{t('Chargement…')}</p>
        {:else if !Object.keys(vars).length}
          <p class="muted">{t('Pas encore de données.')}</p>
        {:else}
          <dl class="kv vars">
            {#each varRows as [k, v] (k)}<dt>{k}</dt><dd>{v}</dd>{/each}
          </dl>
        {/if}
      </section>
    {/if}

    {#if admin && id}
      <LogHealth serverId={id} />
    {/if}

    {#if admin && vps && Object.keys(vps).length}
      <section class="card s-box" aria-label={t('Machine du monitor')}>
        <h2 class="label">{t('Machine du monitor')}</h2>
        <div class="s-tiles three">
          <div><b style:color={col(vps.disk_pct, 80, 90)}>{gauge(vps.disk_pct) ?? '—'} %</b><span>{t('disque utilisé')}</span></div>
          <div><b style:color={col(vps.mem_pct, 80, 90)}>{gauge(vps.mem_pct) ?? '—'} %</b><span>{t('mémoire utilisée')}</span></div>
          <div><b style:color={col(vps.load_per_cpu, 1, 2)}>{vps.load_per_cpu == null || vps.load_per_cpu < 0 ? '—' : vps.load_per_cpu.toFixed(2)}</b><span>{t('charge par cœur')}</span></div>
        </div>
      </section>
    {/if}
  </div>
{/if}

<style>
  .kpis > div > span:first-child { font-size: 11px; font-weight: 800; letter-spacing: .05em; text-transform: uppercase; color: var(--text-2); }
  .kpis b { font-size: 24px; margin: 2px 0; }
  .three { grid-template-columns: repeat(3, minmax(0, 1fr)); }
  .days { display: flex; gap: 3px; height: 28px; }
  .days span { flex: 1; border-radius: 3px; background: var(--ok); }
  .days span.bad { background: var(--warn); }
  .days span.worse { background: var(--danger); }
  .out { border-top: 1px solid var(--line-soft); padding-top: 8px; }
  .label + .out { border-top: 0; padding-top: 0; }
  summary { display: flex; align-items: center; gap: 10px; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  .dot { width: 9px; height: 9px; flex: none; border-radius: 5px; background: var(--muted); }
  .dot.on { background: var(--danger); }
  .ot { display: flex; flex-direction: column; }
  .ot .muted { font-size: 12px; }
  pre { margin: 8px 0 0; padding: 10px; border-radius: 10px; background: #0A0D12; font-size: 11px; white-space: pre-wrap; word-break: break-word; max-height: 40vh; overflow: auto; }
  .kv { display: grid; grid-template-columns: minmax(96px, auto) minmax(0, 1fr); gap: 6px 14px; margin: 0; font-size: 14px; }
  .kv dt { color: var(--muted); font-size: 13px; }
  .kv dd { margin: 0; min-width: 0; overflow-wrap: anywhere; font-weight: 600; }
  .vars { max-height: 360px; overflow: auto; font-size: 13px; }
  .vars dt { font-family: ui-monospace, monospace; font-size: 12px; color: var(--text-2); }
  .vars dd { font-weight: 500; }
  .errs { display: flex; flex-direction: column; max-height: 360px; overflow: auto; }
  .err { display: flex; flex-direction: column; gap: 3px; padding: 8px 0; border-top: 1px solid var(--line-soft); }
  .err:first-child { border-top: 0; padding-top: 0; }
  .err code { font-size: 12px; overflow-wrap: anywhere; color: var(--text-2); }
</style>
