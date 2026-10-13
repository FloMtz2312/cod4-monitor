<script>
  // One match: map, score and the winner of each round, scoreboard by team (MVP), then each round in detail
  // (first blood, bomb, clutch, every kill).
  import Icon from '../components/Icon.svelte';
  import CodName from '../components/CodName.svelte';
  import DataTable from '../components/DataTable.svelte';
  import { get } from '../lib/api.js';
  import { app, mapName } from '../lib/state.svelte.js';
  import { route, back, href } from '../lib/router.svelte.js';
  import { shareCard } from '../lib/card.js';
  import { ago, teamName, weaponName, clean, dateTime, fmt } from '../lib/format.js';
  import { meOn } from '../lib/me.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const mid = $derived(route.params[0] ?? '');
  const id = $derived(app.serverId);
  const me = $derived(meOn(id));
  let d = $state(null);
  let failed = $state('');
  let img = $state(true);

  $effect(() => {
    const sid = id, m = mid;
    if (!sid || !m) return;
    let alive = true;
    d = null;
    failed = '';
    img = true;
    get(`/api/servers/${sid}/match?id=${encodeURIComponent(m)}`).then(r => { if (alive) d = r; }).catch(e => { if (alive) failed = e.message; });
    return () => { alive = false; };
  });

  const m = $derived(d?.match);
  const secs = $derived(m ? Math.max(0, (m.ended_at ?? m.started_at) - m.started_at) : 0);
  const clock = s => `${Math.floor(s / 60)}:${String(Math.floor(s % 60)).padStart(2, '0')}`;
  const official = $derived((d?.players ?? []).some(p => p.score != null));
  const mvp = $derived.by(() => {
    const ps = [...(d?.players ?? [])];
    ps.sort((a, b) => official ? (b.score ?? 0) - (a.score ?? 0) : (b.kills - a.kills) || (a.deaths - b.deaths));
    return ps[0]?.kills > 0 ? ps[0].key : '';
  });
  const teams = $derived.by(() => {
    const ps = d?.players ?? [];
    const sort = (a, b) => official ? (b.score ?? 0) - (a.score ?? 0) : (b.kills - a.kills) || (a.deaths - b.deaths);
    return [
      { id: 'allies', name: teamName('allies'), color: 'var(--allies)', score: d?.score_allies },
      { id: 'axis', name: teamName('axis'), color: 'var(--axis)', score: d?.score_axis },
      { id: '', name: t('Équipe inconnue'), color: 'var(--muted)', score: null }
    ].map(g => ({ ...g, players: ps.filter(p => (p.team === 'allies' || p.team === 'axis' ? p.team : '') === g.id).sort(sort) }))
      .filter(g => g.players.length);
  });
  const result = $derived(d?.winner === 'allies' || d?.winner === 'axis' ? t('Victoire {team}', { team: teamName(d.winner) }) : d?.winner === 'tie' ? t('Égalité') : '');

  const EVENT = { plant: 'Bombe posée', defuse: 'Bombe désamorcée', clutch: 'Clutch', hq_capture: 'QG capturé', hq_destroy: 'QG détruit', flag: 'Drapeau capturé' };
  const events = r => String(r.events ?? '').split('|').filter(Boolean).map(e => {
    const [type, name, value] = e.split(':');
    return { type, name, value: Number(value) };
  }).filter(e => EVENT[e.type]);
  const EVENT_ICON = { plant: '💣', defuse: '✂️', clutch: '🦸', hq_capture: '🚩', hq_destroy: '💥', flag: '🏳' };
  const timeline = rid => (d?.timeline ?? []).filter(e => e.round_id === rid);

  // PC: the full scoreboard
  const kd = x => (x.kills ?? 0) / Math.max(1, x.deaths ?? 0);
  const pct = (a, b) => (b ? `${Math.round((100 * (a ?? 0)) / b)} %` : '—');
  const COLS = $derived([
    { k: 'level', label: t('Niveau'), v: x => (x.level == null ? null : (x.prestige ?? 0) * 100 + x.level), f: x => (x.level == null ? '—' : (x.prestige ? `P${x.prestige} · ` : '') + x.level) },
    { k: 'team', label: t('Équipe'), text: true, v: x => (x.team === 'allies' || x.team === 'axis' ? teamName(x.team) : null) },
    ...(official ? [{ k: 'score', label: t('Points'), title: t('points du mod (fin de match)'), v: x => x.score, strong: true }] : []),
    { k: 'kills', label: t('Kills'), v: x => x.kills ?? 0, strong: !official },
    { k: 'deaths', label: t('Morts'), v: x => x.deaths ?? 0 },
    { k: 'kd', label: 'K/D', v: kd, f: x => kd(x).toFixed(2) },
    { k: 'hs', label: 'HS', title: t('Headshots'), v: x => x.headshots ?? 0, f: x => `${x.headshots ?? 0} (${pct(x.headshots, x.kills)})` },
    { k: 'acc', label: t('Précision'), v: x => (x.shots ? x.hits / x.shots : null), f: x => (x.shots ? pct(x.hits, x.shots) : '—') },
    { k: 'dmg', label: t('Dégâts'), v: x => x.dmg_done, f: x => (x.dmg_done == null ? '—' : fmt(x.dmg_done)) },
    { k: 'ns', label: 'NS', title: t('Noscopes'), v: x => x.noscopes },
    { k: 'qs', label: 'QS', title: t('Quickscopes'), v: x => x.quickscopes },
    { k: 'wb', label: 'WB', title: t('Wallbangs'), v: x => x.wallbangs },
    { k: 'longest', label: t('Plus long'), v: x => x.longest, f: x => (x.longest == null ? '—' : `${Math.round(x.longest)} m`) },
    { k: 'tk', label: t('Kills équipe'), v: x => x.teamkills ?? 0 },
    { k: 'sui', label: t('Suicides'), v: x => x.suicides ?? 0 }
  ]);
</script>

<button class="back" onclick={() => back('matchs')}><Icon name="back" />{t('Retour')}</button>

{#if failed}
  <section class="card pad"><p>{failed}</p></section>
{:else if !d}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <section class="card hero">
    <div class="top">
      {#if img}<img src={`/maps/${m.map}.jpg`} alt="" onerror={() => (img = false)} />{/if}
      <div class="ttl">
        <span class="meta">{(m.gametype || '').toUpperCase()} · {dateTime(m.started_at)} · {t('il y a {d}', { d: ago(m.ended_at || m.started_at) })} · {clock(secs)} · {t('{n} kills', { n: fmt(m.kills) })}</span>
        <h1>{mapName(m.map)}</h1>
      </div>
    </div>
    {#if d.score_allies != null}
      <div class="score">
        <span><small>{teamName('allies').toUpperCase()}</small><b class="allies">{d.score_allies}</b></span>
        <span class="dash">–</span>
        <span class="r"><small>{teamName('axis').toUpperCase()}</small><b class="axis">{d.score_axis}</b></span>
      </div>
      {#if result}<p class="result">{result}</p>{/if}
    {/if}
    {#if d.rounds?.some(r => r.winner)}
      <div class="strip" aria-label={t('Vainqueur de chaque round')}>
        {#each d.rounds as r, i (r.id)}<span class={r.winner ?? ''} title={t('Round {n}', { n: i + 1 }) + (r.winner ? ' · ' + teamName(r.winner) : '')}></span>{/each}
      </div>
      <p class="legend">{t('Un trait par round · bleu Alliés · rouge Axe')}</p>
    {/if}
    <button class="btn card-btn" onclick={() => shareCard(id, 'match', mid)}><Icon name="image" size={18} />{t('Carte Discord')}</button>
  </section>

  {#snippet who(p)}<a class="tn" href={href('joueur', p.key)}><span class="sw" style:background={p.team === 'allies' ? 'var(--allies)' : p.team === 'axis' ? 'var(--axis)' : 'var(--muted)'}></span><CodName raw={p.name_raw} />{#if p.key === mvp}<span class="mvp">MVP</span>{/if}</a>{/snippet}
  <section class="card full" aria-label={t('Tableau des scores')}>
    <h2 class="label">{t('Tableau des scores')}</h2>
    <DataTable rows={d.players ?? []} cols={COLS} name={who} sort={official ? 'score' : 'kills'} keyOf={p => p.key} mine={me?.key ?? ''} label={t('Tableau des scores')} />
  </section>

  <section class="board" aria-label={t('Tableau des scores')}>
    {#each teams as g (g.id)}
      <div class="card team">
        <div class="th">
          <span class="sw" style:background={g.color}></span><b>{g.name}</b>
          <span class="cols" aria-hidden="true"><span>K</span><span>M</span><span>{official ? t('Score') : 'K/D'}</span></span>
        </div>
        {#each g.players as p (p.key)}
          <a class="row" class:mine={p.key === me?.key} href={href('joueur', p.key)}>
            <span class="nm"><CodName raw={p.name_raw} />{#if p.key === mvp}<span class="mvp">MVP</span>{/if}</span>
            <span class="n">{p.kills}</span>
            <span class="n">{p.deaths}</span>
            <span class="n strong">{official ? p.score ?? '—' : (p.kills / Math.max(1, p.deaths)).toFixed(2)}</span>
          </a>
        {/each}
      </div>
    {/each}
  </section>

  <section aria-label={t('Rounds')}>
    <h2 class="label sec">{t('Rounds')}</h2>
    <div class="card rounds">
      {#each d.rounds as r, i (r.id)}
        <details>
          <summary>
            <span class="rn">{i + 1}</span>
            <span class="rw {r.winner ?? ''}">{r.winner === 'allies' || r.winner === 'axis' ? teamName(r.winner) : '—'}</span>
            {#if r.score_allies != null}<span class="rs"><span class="allies">{r.score_allies}</span>–<span class="axis">{r.score_axis}</span></span>{/if}
            <span class="rm muted">{#if r.first_blood}{t('Premier sang')} <CodName raw={r.first_blood} />{/if}{#if r.top_killer}{' · '}{t('Meilleur tueur')} <CodName raw={r.top_killer} />{/if}</span>
            <span class="muted rk">{#each events(r) as e, j (j)}<span class="evi" title={t(EVENT[e.type])}>{EVENT_ICON[e.type]}</span>{/each}{t('{n} kills', { n: r.kills })}{#if r.ended_at}{' · '}{clock(r.ended_at - r.started_at)}{/if}</span>
            <Icon name="chevronDown" size={18} />
          </summary>
          <div class="rd">
            {#if r.first_blood}<p><span class="muted">{t('Premier sang')}</span> <CodName raw={r.first_blood} /></p>{/if}
            {#if r.top_killer}<p><span class="muted">{t('Meilleur tueur')}</span> <CodName raw={r.top_killer} /></p>{/if}
            {#each events(r) as e, j (j)}
              <p class="ev"><span class="tag">{t(EVENT[e.type])}{#if e.type === 'clutch'}{' '}1v{e.value}{/if}</span> <CodName raw={e.name} /></p>
            {/each}
            <ol class="tl">
              {#each timeline(r.id) as e (e.type + e.id)}
                <li>
                  <time>+{clock(Math.max(0, e.ts - r.started_at))}</time>
                  {#if e.type === 'kill'}
                    <span class="k">
                      {#if e.actor}<CodName raw={e.actor} />{:else}<span class="muted">{t('mort seul')}</span>{/if}
                      <span class="w">{weaponName(e.weapon)}{#if e.cause === 'MOD_HEAD_SHOT'}{' · HS'}{/if}{#if e.teamkill}{' · '}{t('allié')}{/if}</span>
                      {#if e.actor && clean(e.actor) !== clean(e.target)}<CodName raw={e.target} />{/if}
                    </span>
                  {:else}
                    <span class="k"><span class="tag">{t(EVENT[e.type] ?? e.type)}</span> <CodName raw={e.actor} /></span>
                  {/if}
                </li>
              {:else}
                <li class="muted">{t('Aucun événement enregistré.')}</li>
              {/each}
            </ol>
          </div>
        </details>
      {/each}
    </div>
    {#if d.timeline_truncated}<p class="muted trunc">{t('Affichage limité aux 5 000 premiers événements du match.')}</p>{/if}
  </section>
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 4px; height: 40px; padding: 0 10px 0 4px; margin: -4px 0 8px -4px; border: 0; background: transparent; color: var(--text-2); font-weight: 700; }
  .pad { padding: 16px; }
  .hero { overflow: hidden; padding-bottom: 14px; }
  .top { position: relative; min-height: 132px; display: flex; align-items: flex-end; padding: 16px; }
  .top img { position: absolute; inset: 0; width: 100%; height: 100%; object-fit: cover; opacity: .45; }
  .top::after { content: ''; position: absolute; inset: 0; background: linear-gradient(180deg, transparent 20%, var(--surface)); }
  .ttl { position: relative; z-index: 1; }
  .meta { font-size: 12px; font-weight: 800; color: var(--text-2); letter-spacing: .04em; }
  h1 { font-family: var(--font-display); font-size: 30px; font-weight: 700; }
  .score { display: flex; align-items: center; justify-content: space-between; padding: 4px 16px 0; }
  .score span { display: flex; flex-direction: column; }
  .score .r { align-items: flex-end; }
  .score small { font-size: 12px; font-weight: 800; color: var(--text-2); letter-spacing: .06em; }
  .score b { font-family: var(--font-display); font-size: 40px; line-height: 1; }
  .dash { font-size: 28px; color: var(--muted); }
  .allies { color: var(--allies); }
  .axis { color: var(--axis); }
  .result { margin: 6px 16px 0; text-align: center; font-weight: 800; font-size: 14px; }
  .strip { display: flex; gap: 3px; margin: 12px 16px 0; }
  .strip span { flex: 1; height: 8px; border-radius: 3px; background: #33405A; }
  .strip span.allies { background: var(--allies); }
  .strip span.axis { background: var(--axis); }
  .legend { margin: 6px 16px 0; font-size: 11px; color: var(--muted); }
  .card-btn { margin: 12px 16px 0; width: calc(100% - 32px); }
  .board { display: grid; gap: 12px; margin-top: 12px; }
  .full { display: none; padding: 14px 16px; margin-top: 12px; flex-direction: column; gap: 8px; }
  .tn { display: flex; align-items: center; gap: 6px; min-width: 0; color: var(--text); text-decoration: none; }
  .tn .sw { flex: none; }
  .rs { font-family: var(--font-display); font-weight: 700; font-variant-numeric: tabular-nums; }
  .evi { margin-right: 6px; }
  .rm { display: none; align-items: center; gap: 4px; font-size: 13px; min-width: 0; white-space: nowrap; overflow: hidden; }
  .trunc { font-size: 12px; margin: 8px 2px 0; }
  .team { overflow: hidden; }
  .th { display: flex; align-items: center; gap: 8px; padding: 12px 14px 8px; }
  .sw { width: 10px; height: 10px; border-radius: 3px; }
  .cols { margin-left: auto; display: flex; font-size: 11px; color: var(--muted); font-weight: 700; }
  .cols span { width: 40px; text-align: right; }
  .cols span:last-child { width: 56px; }
  .row { display: flex; align-items: center; min-height: 48px; padding: 0 14px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); font-size: 14px; }
  .row.mine { background: var(--accent-soft); }
  .nm { flex: 1; min-width: 0; display: flex; align-items: center; gap: 6px; font-weight: 700; white-space: nowrap; overflow: hidden; }
  .mvp { flex: none; font-size: 10px; font-weight: 800; padding: 2px 6px; border-radius: 6px; background: var(--accent); color: var(--on-accent); }
  .n { width: 40px; text-align: right; font-variant-numeric: tabular-nums; color: var(--text-2); }
  .n.strong { width: 56px; font-weight: 800; color: var(--text); }
  .sec { display: block; margin: 16px 2px 8px; }
  .rounds { overflow: hidden; }
  details { border-top: 1px solid var(--line-soft); }
  details:first-child { border-top: 0; }
  summary { display: flex; align-items: center; gap: 10px; min-height: 50px; padding: 0 14px; cursor: pointer; list-style: none; }
  summary::-webkit-details-marker { display: none; }
  details[open] summary :global(svg) { transform: rotate(180deg); }
  .rn { width: 26px; height: 26px; border-radius: 8px; background: var(--surface-2); display: flex; align-items: center; justify-content: center; font-weight: 800; font-size: 12px; }
  .rw { font-weight: 800; font-size: 14px; }
  .rw.allies { color: var(--allies); }
  .rw.axis { color: var(--axis); }
  .rk { flex: 1; font-size: 13px; text-align: right; }
  .rd { padding: 0 14px 12px; font-size: 14px; }
  .rd p { margin: 0 0 6px; display: flex; flex-wrap: wrap; gap: 4px; align-items: center; }
  .tag { font-size: 11px; font-weight: 800; padding: 2px 6px; border-radius: 6px; background: var(--accent-soft); color: var(--accent); }
  .tl { list-style: none; margin: 6px 0 0; padding: 0; display: flex; flex-direction: column; gap: 6px; }
  .tl li { display: flex; gap: 10px; align-items: baseline; }
  .tl time { flex: none; width: 44px; font-size: 12px; color: var(--muted); font-variant-numeric: tabular-nums; }
  .k { display: flex; flex-wrap: wrap; gap: 6px; align-items: baseline; min-width: 0; }
  .w { font-size: 12px; color: var(--muted); }
  @media (min-width: 960px) {
    .board { display: none; }
    .full { display: flex; }
    .rm { display: flex; margin-left: 12px; }
  }
</style>
