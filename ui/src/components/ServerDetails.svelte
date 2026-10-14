<script>
  // The server at a glance: latency, players' average ping, availability over 30 days, and its details (unfolded on demand).
  import CodName from './CodName.svelte';
  import Icon from './Icon.svelte';
  import { mapName } from '../lib/state.svelte.js';
  import { agoText, pingColor } from '../lib/format.js';
  import { dt } from '../lib/dates.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ status: any, uptime: any, now: number }} */
  let { status, uptime, now } = $props();

  const snap = $derived((Array.isArray(status?.snapshot) ? status.snapshot[0] : status?.snapshot) ?? {});
  const vars = $derived(status?.vars ?? {});
  const on = $derived(snap.online === 1 || snap.online === true);
  const lastOk = $derived(status?.last_ok?.[0]?.ts ?? null);
  const pings = $derived((status?.players ?? []).map(p => p.ping).filter(p => p != null && p > 0 && p < 999));
  const avgPing = $derived(pings.length ? Math.round(pings.reduce((a, b) => a + b, 0) / pings.length) : null);
  const up = $derived(uptime?.['30d']);
  const rows = $derived([
    [t('Statut'), on ? t('En ligne') : snap.ts ? t('Hors ligne') : '—'],
    [t('Map'), snap.map || vars.mapname ? `${mapName(snap.map || vars.mapname)} (${snap.map || vars.mapname})` : '—'],
    [t('Mode'), snap.gametype || vars.g_gametype || '—'],
    [t('Joueurs'), on ? `${snap.players ?? 0} / ${snap.max_players ?? '?'}` : '—'],
    [t('Latence'), on && snap.latency_ms != null ? `${snap.latency_ms.toFixed(1)} ms` : '—'],
    [t('Dernier sondage OK'), lastOk ? `${dt(lastOk)} (${agoText(lastOk, now)})` : '—'],
    [t('Map lancée'), vars.g_mapStartTime || '—'],
    [t('Mod'), vars.fs_game || '—'],
    [t('Version'), vars.version || vars.shortversion || '—']
  ]);
</script>

{#if status}
  <section class="card srv" aria-label={t('Serveur')}>
    <div class="tiles">
      <div><b>{on && snap.latency_ms != null ? `${Math.round(snap.latency_ms)} ms` : '—'}</b><span>{t('latence')}</span></div>
      <div><b style:color={avgPing == null ? null : pingColor(avgPing)}>{avgPing == null ? '—' : `${avgPing} ms`}</b><span>{t('ping moyen')}</span></div>
      <div><b>{up == null ? '—' : `${up >= 99.95 ? 100 : up.toFixed(1)} %`}</b><span>{t('dispo 30 j')}</span></div>
    </div>
    <details>
      <summary><span>{t('Détails du serveur')}</span><Icon name="chevronDown" size={18} /></summary>
      <dl>
        <dt>{t('Nom')}</dt><dd class="host"><CodName raw={snap.hostname || vars.sv_hostname || '—'} /></dd>
        {#each rows as [k, v] (k)}<dt>{k}</dt><dd>{v}</dd>{/each}
      </dl>
    </details>
  </section>
{/if}

<style>
  .srv { padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; }
  .tiles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .tiles div { display: flex; flex-direction: column; padding: 8px 10px; border-radius: 12px; background: var(--surface-2); min-width: 0; }
  .tiles b { font-family: var(--font-display); font-size: 19px; font-variant-numeric: tabular-nums; white-space: nowrap; }
  .tiles span { font-size: 12px; color: var(--muted); }
  summary { display: flex; align-items: center; justify-content: space-between; height: 36px; cursor: pointer; font-size: 13px; font-weight: 800; color: var(--text-2); list-style: none; }
  summary::-webkit-details-marker { display: none; }
  details[open] summary :global(svg) { transform: rotate(180deg); }
  dl { margin: 4px 0 0; display: grid; grid-template-columns: auto minmax(0, 1fr); gap: 6px 14px; font-size: 13px; }
  dt { color: var(--muted); font-weight: 700; }
  dd { margin: 0; overflow-wrap: anywhere; }
  .host { display: flex; min-width: 0; font-weight: 700; }
</style>
