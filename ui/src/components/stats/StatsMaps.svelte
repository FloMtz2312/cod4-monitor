<script>
  // Maps: ranking with a verdict per map (compared with the server average), retention of the players,
  // S&D rounds per map and the end-of-match votes.
  import { get } from '../../lib/api.js';
  import { mapName } from '../../lib/state.svelte.js';
  import { t } from '../../lib/i18n.svelte.js';
  import { dur, pct, dec } from './util.js';

  /** @type {{ id: number, period: string, mod: boolean }} */
  let { id, period, mod } = $props();

  let rank = $state(null);
  let stats = $state(null);
  let rounds = $state(null);
  let votes = $state(null);
  let sort = $state('player_secs');

  $effect(() => {
    const sid = id, p = period;
    let alive = true;
    rank = stats = rounds = votes = null;
    const load = (path, set) => get(`/api/servers/${sid}/${path}?range=${p}`).then(r => { if (alive) set(r); }).catch(() => { if (alive) set({}); });
    load('maprank', r => (rank = r.maps ?? []));
    load('mapstats', r => (stats = r));
    load('maprounds', r => (rounds = r.maps ?? []));
    load('mapvotes', r => (votes = r.votes ?? []));
    return () => { alive = false; };
  });

  /** @type {Record<string, [string, string]>} */
  const VERDICT = {
    keep: ['✓ À garder', 'Autant de joueurs que la moyenne du serveur, ou plus'],
    mid: ['! Moyenne', 'Un peu moins de joueurs que la moyenne'],
    drop: ['✕ À remplacer', 'Beaucoup moins de joueurs que la moyenne, ou les joueurs partent'],
    few: ['? Peu de données', 'Pas assez jouée sur la période pour juger'],
    never: ['✕ Jamais lancée', 'Proposée au vote mais jamais jouée sur la période']
  };
  function verdict(x, avg) {
    if (!x.plays) return 'never';
    if (x.plays < 2 || x.secs < 1800 || !avg) return 'few';
    const r = x.avg_players / avg;
    let v = r >= 0.9 ? 'keep' : r >= 0.6 ? 'mid' : 'drop';
    if (x.avg_change != null && x.avg_change <= -1) v = v === 'keep' ? 'mid' : 'drop'; // players leave during it
    return v;
  }

  const all = $derived((rank ?? []).filter(x => x.map));
  const played = $derived(all.filter(x => x.plays));
  const totalSecs = $derived(played.reduce((a, x) => a + x.secs, 0));
  const avg = $derived(totalSecs ? played.reduce((a, x) => a + x.player_secs, 0) / totalSecs : 0);
  const top = $derived(played.slice().sort((a, b) => b.player_secs - a.player_secs)[0]);
  const byRound = $derived(new Map((rounds ?? []).map(r => [r.map, r])));
  const rows = $derived.by(() => {
    const k = sort;
    return all.map(x => ({ ...x, verdict: verdict(x, avg), r: byRound.get(x.map) }))
      .sort((a, b) => (k === 'map' ? a.map.localeCompare(b.map) : (b[k] - a[k]) || a.map.localeCompare(b.map)));
  });
  const count = k => rows.filter(x => x.verdict === k).length;
  const maxAvg = $derived(Math.max(avg, ...all.map(x => x.avg_players ?? 0)) || 1);
  const change = v => (v == null ? null : Math.round(v * 10) / 10 || 0); // no "-0.0"
  const changeCol = v => (v == null ? 'inherit' : v >= 0.5 ? 'var(--ok)' : v <= -0.5 ? 'var(--danger-text)' : 'inherit');
  const SORTS = [['player_secs', 'Fréquentation'], ['avg_players', 'Joueurs'], ['plays', 'Passages'], ['secs', 'Temps'], ['map', 'Nom']];

  /** Attack / defense share of a map's S&D rounds; "unbalanced" when a side wins ≥ 65 % over ≥ 10 rounds. */
  function split(r) {
    const n = r ? r.attack + r.defense : 0;
    if (!n) return null;
    const a = Math.round((100 * r.attack) / n);
    return { a, bad: r.rounds >= 10 && Math.max(a, 100 - a) >= 65 };
  }

  // votes per map, FFA and S&D apart
  const voteRows = $derived(votes ?? []);
</script>

<div class="s-grid">
  <section class="card s-box s-wide" aria-label={t('Maps de la période')}>
    <h2 class="label">{t('Maps de la période')}</h2>
    {#if rank === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else}
      <div class="s-tiles">
        <div><b>{played.length}</b><span>{t('maps jouées')}</span></div>
        <div><b>{dec(avg)}</b><span>{t('joueurs en moyenne sur le serveur')}</span></div>
        <div><b>{top ? mapName(top.map) : '—'}</b><span>{t('la plus fréquentée')}</span></div>
        <div><b class="ok">{count('keep')}</b><span>{t('à garder')}</span></div>
        <div><b class="bad">{count('drop') + count('never')}</b><span>{t('à remplacer')}</span></div>
      </div>
      <p class="muted s-small legend">
        {t('Chaque map est comparée à la moyenne du serveur : quand elle tourne, y a-t-il autant de joueurs que d\'habitude ?')}
        <span class="s-verdict keep">{t('✓ À garder')}</span> {t('au moins 90 % de la moyenne')} ·
        <span class="s-verdict mid">{t('! Moyenne')}</span> {t('60 à 90 %')} ·
        <span class="s-verdict drop">{t('✕ À remplacer')}</span> {t('moins de 60 %, ou les joueurs partent pendant la map')} ·
        <span class="s-verdict few">{t('? Peu de données')}</span> {t('moins de 2 passages ou de 30 min.')}
      </p>
    {/if}
  </section>

  <section class="card s-box s-wide" aria-label={t('Classement des maps')}>
    <div class="s-hd">
      <h2 class="label">{t('Classement des maps')}</h2>
      <div class="chips" role="group" aria-label={t('Trier par')}>
        {#each SORTS as [k, label] (k)}<button aria-pressed={sort === k} class:on={sort === k} onclick={() => (sort = k)}>{t(label)}</button>{/each}
      </div>
    </div>
    {#if rank === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else if !rows.length}
      <p class="muted">{t('Aucune map sur la période')}</p>
    {:else}
      <!-- phone: one card per map -->
      <div class="phone">
        {#each rows as x (x.map)}
          <div class="mcard">
            <span class="thumb" style:background-image={`url(/maps/${encodeURIComponent(x.map)}.jpg)`} aria-hidden="true"></span>
            <div class="mi">
              <div class="mt"><b>{mapName(x.map)}</b><span class="s-verdict {x.verdict}" title={t(VERDICT[x.verdict][1])}>{t(VERDICT[x.verdict][0])}</span></div>
              <div class="barline">
                <span class="mbar" title={t('{avg} joueurs en moyenne, {max} au maximum', { avg: dec(x.avg_players), max: x.max_players })}><i style:width="{(100 * x.avg_players) / maxAvg}%"></i><u style:left="{(100 * avg) / maxAvg}%"></u></span>
                <b>{x.plays ? dec(x.avg_players) : '—'}</b>
              </div>
              <div class="muted s-small">
                {t('{n} passages', { n: x.plays })}{#if x.secs}{' · '}{dur(x.secs)}{' · '}{t('vide {p}', { p: pct(x.empty_secs, x.secs) })}{/if}{#if x.avg_change != null && x.avg_change <= -1}{' · '}<span class="bad">{t('les joueurs partent')}</span>{/if}{#if mod && x.offered}{' · '}{t('vote {c} / {o}', { c: x.chosen, o: x.offered })}{/if}
              </div>
            </div>
          </div>
        {/each}
      </div>
      <!-- computer: the full table -->
      <div class="pc s-scroll">
        <table class="s-table">
          <thead><tr>
            <th class="n">#</th><th>{t('Map')}</th><th>{t('Verdict')}</th>
            <th title={t('nombre moyen de joueurs pendant que la map tourne ; trait = moyenne du serveur')}>{t('Joueurs moy.')} <span class="muted">┆ {t('serveur')}</span></th>
            <th class="n" title={t('nombre de fois où la map a été lancée')}>{t('Passages')}</th>
            <th class="n" title={t('temps total sur la map (part du temps du serveur)')}>{t('Temps joué')}</th>
            <th class="n" title={t('part du temps de cette map sans aucun joueur')}>{t('Serveur vide')}</th>
            <th class="n" title={t('joueurs à la fin moins joueurs au début, en moyenne par passage')}>{t('Évolution')}</th>
            {#if mod}<th class="n" title={t('vote de fin de match : choisie / proposée')}>{t('Vote')}</th><th title={t('part des rounds S&D gagnés par l\'attaque et par la défense')}>{t('Attaque / défense')}</th>{/if}
          </tr></thead>
          <tbody>
            {#each rows as x, i (x.map)}
              {@const s = split(x.r)}
              {@const ch = change(x.avg_change)}
              <tr>
                <td class="n muted">{i + 1}</td>
                <td><span class="mname"><span class="s-thumb" style:background-image={`url(/maps/${encodeURIComponent(x.map)}.jpg)`} aria-hidden="true"></span><b title={x.map}>{mapName(x.map)}</b></span></td>
                <td><span class="s-verdict {x.verdict}" title={t(VERDICT[x.verdict][1])}>{t(VERDICT[x.verdict][0])}</span></td>
                <td title={t('{avg} joueurs en moyenne, {max} au maximum', { avg: dec(x.avg_players), max: x.max_players })}>
                  <span class="barline"><span class="mbar"><i style:width="{(100 * x.avg_players) / maxAvg}%"></i><u style:left="{(100 * avg) / maxAvg}%"></u></span>{x.plays ? dec(x.avg_players) : '—'}</span>
                </td>
                <td class="n">{x.plays}</td>
                <td class="n">{x.secs ? dur(x.secs) : '—'} <span class="muted">{x.secs ? pct(x.secs, totalSecs) : ''}</span></td>
                <td class="n">{x.secs ? pct(x.empty_secs, x.secs) : '—'}</td>
                <td class="n" style:color={changeCol(ch)}>{ch == null ? '—' : `${ch > 0 ? '+' : ''}${dec(ch)}`}</td>
                {#if mod}
                  <td class="n">{x.offered ? `${x.chosen} / ${x.offered}` : '—'}</td>
                  <td class="ad">
                    {#if s}
                      <div class="s-split"><div class="att" style:width="{s.a}%"></div><div class="def" style:flex="1"></div></div>
                      <span class="s-small" class:warn={s.bad}>{s.a} / {100 - s.a}{#if s.bad}{' · '}{t('déséquilibrée')}{/if}</span>
                    {:else}<span class="muted">—</span>{/if}
                  </td>
                {/if}
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>

  <section class="card s-box" aria-label={t('Maps : rétention des joueurs')}>
    <div class="s-hd"><h2 class="label">{t('Maps : rétention des joueurs')}</h2>{#if stats?.source === 'none'}<span class="muted s-small">{t('(départs : nécessite les logs FTP)')}</span>{/if}</div>
    {#if stats === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else if !stats.maps?.length}
      <p class="muted">{t('Aucun match sur la période')}</p>
    {:else}
      <div class="s-scroll">
        <table class="s-table">
          <thead><tr><th>{t('Map')}</th><th class="n">{t('Matchs')}</th><th class="n">{t('Durée moy.')}</th><th class="n">{t('Joueurs / match')}</th><th class="n" title={t('part des joueurs partis avant la fin du match')}>{t('Partis en cours')}</th><th class="n" title={t('joueurs à la fin moins joueurs au début, en moyenne')}>{t('Évolution')}</th></tr></thead>
          <tbody>
            {#each stats.maps as x (x.map)}
              <tr>
                <td><b>{mapName(x.map)}</b></td><td class="n">{x.matches}</td><td class="n">{dur(x.secs / x.matches)}</td>
                <td class="n">{x.avg_players ? dec(x.avg_players) : '—'}</td>
                <td class="n">{x.left_rate == null ? '—' : `${Math.round(100 * x.left_rate)} %`}</td>
                <td class="n" style:color={x.avg_change == null ? null : x.avg_change > 0.05 ? 'var(--ok)' : x.avg_change < -0.05 ? 'var(--danger-text)' : null}>{x.avg_change == null ? '—' : `${x.avg_change > 0 ? '+' : ''}${dec(x.avg_change)}`}</td>
              </tr>
            {/each}
          </tbody>
        </table>
      </div>
    {/if}
  </section>

  {#if mod}
    <section class="card s-box" aria-label={t('Rounds par map')}>
      <div class="s-hd"><h2 class="label">{t('Rounds par map')}</h2><span class="muted s-small">{t('S&D : durée des rounds et des matchs, camp qui gagne')}</span></div>
      {#if rounds === null}
        <p class="muted">{t('Chargement…')}</p>
      {:else if !rounds.length}
        <p class="muted">{t('Aucun round terminé sur la période (nécessite les lignes de fin de round du mod)')}</p>
      {:else}
        {#each rounds as x (x.map)}
          {@const s = split(x)}
          <div class="rr">
            <div class="mt"><b>{mapName(x.map)}</b><span class="muted s-small">{t('{n} rounds', { n: x.rounds })} · {t('{d} en moy.', { d: dur(x.avg_s) })} · {dur(x.min_s)} / {dur(x.max_s)}</span></div>
            {#if s}
              <div class="s-split"><div class="att" style:width="{s.a}%"></div><div class="def" style:flex="1"></div></div>
              <span class="s-small"><span class="att-t">{t('attaque')} {s.a} %</span> · <span class="def-t">{t('défense')} {100 - s.a} %</span>{#if s.bad}{' '}<span class="s-verdict mid">{t('! Déséquilibrée')}</span>{/if}</span>
            {/if}
            {#if x.matches}<span class="muted s-small">{t('{n} matchs de {d} en moy. ({r} rounds)', { n: x.matches, d: dur(x.match_avg_s), r: dec(x.match_rounds) })}</span>{/if}
          </div>
        {/each}
        <p class="muted s-small">{t('Durée d\'un round = d\'une fin de round à la suivante (gel de 5 s et killcam compris).')} <span class="s-verdict mid">{t('! Déséquilibrée')}</span> {t('un camp gagne au moins 65 % des rounds (min. 10 rounds).')}</p>
      {/if}
    </section>

    <section class="card s-box" aria-label={t('Votes de map')}>
      <div class="s-hd"><h2 class="label">{t('Votes de map')}</h2><span class="muted s-small">{t('vote de fin de match')}</span></div>
      {#if votes === null}
        <p class="muted">{t('Chargement…')}</p>
      {:else if !voteRows.length}
        <p class="muted">{t('Aucun vote enregistré sur la période')}</p>
      {:else}
        <div class="s-scroll">
          <table class="s-table">
            <thead><tr><th>{t('Map')}</th><th>{t('Mode')}</th><th class="n">{t('Proposée')}</th><th class="n">{t('Votes')}</th><th class="n">{t('Votes / proposition')}</th><th class="n">{t('Choisie')}</th></tr></thead>
            <tbody>
              {#each voteRows as x (x.map + x.mode)}
                <tr><td><b>{mapName(x.map)}</b></td><td class="muted">{x.mode === 'dm' ? 'FFA' : 'S&D'}</td><td class="n">{x.offered}</td><td class="n">{x.votes ?? 0}</td>
                  <td class="n">{x.offered ? dec((x.votes ?? 0) / x.offered) : '—'}</td><td class="n">{x.chosen ?? 0}</td></tr>
              {/each}
            </tbody>
          </table>
        </div>
      {/if}
    </section>
  {/if}
</div>

<style>
  .ok { color: var(--ok); }
  .bad { color: var(--danger-text); }
  .warn { color: var(--warn); font-weight: 700; }
  .legend { line-height: 1.9; }
  .chips { display: flex; flex-wrap: wrap; gap: 6px; }
  .chips button { height: 32px; padding: 0 12px; border-radius: 16px; border: 1px solid var(--line); background: transparent; color: var(--text-2); font-size: 13px; font-weight: 700; }
  .chips button.on { background: var(--accent-soft); border-color: var(--accent); color: var(--accent); }
  .mcard { display: flex; gap: 12px; align-items: center; padding: 10px 0; border-top: 1px solid var(--line-soft); }
  .mcard:first-child { border-top: 0; padding-top: 0; }
  .thumb { width: 56px; height: 56px; flex: none; border-radius: 12px; background: var(--surface-2) center / cover no-repeat; }
  .mi { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 5px; }
  .mt { display: flex; justify-content: space-between; align-items: center; gap: 8px; font-size: 15px; }
  .mt b { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .barline { display: flex; align-items: center; gap: 8px; }
  .mbar { position: relative; flex: 1; min-width: 80px; height: 8px; border-radius: 4px; background: var(--surface-2); }
  .mbar i { position: absolute; left: 0; top: 0; bottom: 0; border-radius: 4px; background: var(--accent); }
  .mbar u { position: absolute; top: -3px; bottom: -3px; width: 0; border-left: 2px dotted var(--text-2); }
  td .mbar { width: 90px; min-width: 0; flex: none; }
  .mname { display: inline-flex; align-items: center; gap: 10px; }
  .ad { min-width: 110px; }
  .ad .s-split { margin-bottom: 3px; }
  .rr { display: flex; flex-direction: column; gap: 5px; padding-top: 10px; border-top: 1px solid var(--line-soft); }
  .s-hd + .rr { border-top: 0; padding-top: 0; }
  .att-t { color: var(--axis); }
  .def-t { color: var(--allies); }
  .pc { display: none; }
  @media (min-width: 960px) {
    .phone { display: none; }
    .pc { display: block; }
  }
</style>
