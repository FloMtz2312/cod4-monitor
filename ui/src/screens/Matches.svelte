<script>
  // Matchs: the matches of the period, newest first (map, mode, start, length, kills, score, winner, top killer);
  // second tab (#/matchs/rounds): the rounds by map and the latest rounds.
  import CodName from '../components/CodName.svelte';
  import Seg from '../components/Seg.svelte';
  import Icon from '../components/Icon.svelte';
  import RoundsView from '../components/RoundsView.svelte';
  import { get } from '../lib/api.js';
  import { app, mapName } from '../lib/state.svelte.js';
  import { href, route, replace } from '../lib/router.svelte.js';
  import { ago, teamName, dateTime, fmt } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const tab = $derived(route.params[0] === 'rounds' ? 'rounds' : 'matchs');
  let period = $state('7d');
  let list = $state(null);
  let failed = $state(false);

  $effect(() => {
    const sid = id, p = period;
    if (!sid || tab !== 'matchs') return;
    let alive = true;
    list = null;
    failed = false;
    get(`/api/servers/${sid}/matches?range=${p}`)
      .then(r => { if (alive) list = r.matches ?? []; })
      .catch(() => { if (alive) { list = []; failed = true; } });
    return () => { alive = false; };
  });

  const duration = m => {
    const s = Math.max(0, (m.ended_at ?? m.started_at) - m.started_at);
    return s < 3600 ? `${Math.max(1, Math.round(s / 60))} min` : `${Math.floor(s / 3600)} h ${String(Math.round((s % 3600) / 60)).padStart(2, '0')}`;
  };
  const result = x => x.winner === 'allies' || x.winner === 'axis' ? t('victoire {team}', { team: teamName(x.winner) }) : x.winner === 'tie' ? t('égalité') : '';
</script>

<h1 class="title">{t('Matchs')}</h1>
<Seg label={t('Vue')} value={tab} onchange={v => replace('matchs', ...(v === 'rounds' ? ['rounds'] : []))} items={[['matchs', t('Matchs')], ['rounds', t('Rounds')]]} />
<div class="per">
  <Seg label={t('Période')} value={period} onchange={v => (period = v)} items={[['24h', t('24 h')], ['7d', t('7 jours')], ['30d', t('30 jours')], ['90d', t('90 jours')]]} />
</div>

{#if tab === 'rounds'}
  <RoundsView sid={id} {period} />
{:else if list === null}
  <p class="muted pad">{t('Chargement…')}</p>
{:else if failed}
  <p class="muted pad">{t('Matchs indisponibles.')}</p>
{:else if !list.length}
  <section class="card empty"><p class="muted">{t('Aucun match sur cette période.')}</p></section>
{:else}
  <div class="list">
    {#each list as x (x.match.id)}
      <a class="card m" href={href('match', String(x.match.id))}>
        <span class="thumb" style:background-image={`url(/maps/${x.match.map}.jpg)`} aria-hidden="true"></span>
        <span class="info">
          <b>{mapName(x.match.map)}</b>
          <span class="muted">{(x.match.gametype || '').toUpperCase()} · {duration(x.match)} · {t('{n} rounds', { n: x.match.rounds })} · {t('{n} kills', { n: fmt(x.match.kills) })}</span>
          <span class="muted" title={t('il y a {d}', { d: ago(x.match.started_at) })}>{dateTime(x.match.started_at)}{#if x.players}{' · '}{t('{n} joueurs', { n: x.players })}{/if}</span>
        </span>
        <span class="res">
          {#if x.score_allies != null}
            <span class="score"><span class="allies">{x.score_allies}</span>–<span class="axis">{x.score_axis}</span></span>
            <span class="win">{result(x)}</span>
          {/if}
          {#if x.top_name}<span class="top">{t('Top')} <CodName raw={x.top_name} /> · {x.top_kills}</span>{/if}
        </span>
        <Icon name="chevronRight" size={18} />
      </a>
    {/each}
  </div>
{/if}

<style>
  .title { margin-bottom: 14px; }
  .pad { margin-top: 14px; }
  .per { margin-top: 8px; }
  .empty { padding: 16px; margin-top: 14px; }
  .empty p { margin: 0; }
  .list { display: grid; gap: 10px; margin-top: 14px; }
  .m { display: flex; align-items: center; gap: 12px; padding: 10px 12px 10px 10px; text-decoration: none; color: var(--text); min-width: 0; }
  .thumb { width: 64px; height: 64px; flex: none; border-radius: 14px; background: var(--surface-2) center / cover no-repeat; }
  .info { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 1px; }
  .info b { font-size: 16px; }
  .info .muted { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .res { display: flex; flex-direction: column; align-items: flex-end; gap: 2px; min-width: 0; max-width: 45%; }
  .score { font-family: var(--font-display); font-size: 22px; font-weight: 700; font-variant-numeric: tabular-nums; }
  .allies { color: var(--allies); }
  .axis { color: var(--axis); }
  .win { font-size: 12px; color: var(--text-2); font-weight: 700; }
  .top { display: flex; gap: 4px; font-size: 12px; color: var(--muted); white-space: nowrap; overflow: hidden; max-width: 100%; }
  @media (min-width: 960px) { .list { grid-template-columns: repeat(2, minmax(0, 1fr)); } }
</style>
