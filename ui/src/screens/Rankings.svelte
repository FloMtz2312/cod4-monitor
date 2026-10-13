<script>
  // Classements: player of the period, the top players by metric (my place shown even outside the top; on PC the
  // full sortable combat table), the community challenges of the week, records and badges, then links to the
  // matches, the duel and the advanced stats.
  import Icon from '../components/Icon.svelte';
  import CodName from '../components/CodName.svelte';
  import Seg from '../components/Seg.svelte';
  import DataTable from '../components/DataTable.svelte';
  import Challenges from '../components/Challenges.svelte';
  import { get } from '../lib/api.js';
  import { app, mapName } from '../lib/state.svelte.js';
  import { meOn } from '../lib/me.svelte.js';
  import { isOwner } from '../lib/perms.js';
  import { href } from '../lib/router.svelte.js';
  import { fmt, clean, day, weaponName } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const me = $derived(meOn(id));
  const owner = $derived(isOwner(app.me, id));

  let period = $state('7d');
  let metric = $state('kills');
  let limit = $state(10); // top 10, then 50 more at each tap
  let combat = $state(null);
  let records = $state(null);
  /** @type {{ day: any, week: any }} */
  let tops = $state({ day: null, week: null });
  let badges = $state([]);
  let challenges = $state(null);
  let filter = $state('');

  $effect(() => {
    const sid = id, p = period;
    if (!sid) return;
    let alive = true;
    combat = records = null;
    badges = [];
    limit = 10;
    const base = `/api/servers/${sid}`;
    get(`${base}/combat?range=${p}`).then(r => { if (alive) combat = r.players ?? []; }).catch(() => { if (alive) combat = []; });
    get(`${base}/records?range=${p}`).then(r => { if (alive) { records = r.records ?? {}; tops = { day: r.day, week: r.week }; } }).catch(() => { if (alive) records = {}; });
    get(`${base}/badges?range=${p}`).then(r => { if (alive) badges = r.badges ?? []; }).catch(() => {});
    return () => { alive = false; };
  });

  $effect(() => {
    const sid = id;
    if (!sid) return;
    let alive = true;
    challenges = null;
    get(`/api/servers/${sid}/challenges`).then(r => { if (alive) challenges = r; }).catch(() => {});
    return () => { alive = false; };
  });

  const periodWord = $derived({ '7d': t('de la semaine'), '30d': t('du mois'), '90d': t('des 90 jours') }[period]);
  const minKills = $derived(period === '7d' ? 10 : 20);
  const kd = x => (x.kills ?? 0) / Math.max(1, x.deaths ?? 0);
  const hsPct = x => (x.kills ? (100 * (x.headshots ?? 0)) / x.kills : 0);
  const pct = (a, b) => (b ? `${Math.round((100 * (a ?? 0)) / b)} %` : '—');
  const meters = v => (v == null ? '—' : `${Math.round(v)} m`);

  // metrics: the mod's ones only when its data exists
  const metrics = $derived.by(() => {
    const ps = combat ?? [];
    const kills = x => t('{n} kills', { n: fmt(x.kills) });
    /** @type {Record<string, { label: string, val: (x: any) => number, fmt: (v: number) => string, sub: (x: any) => string, min?: boolean }>} */
    const m = {
      kills: { label: t('Kills'), val: x => x.kills ?? 0, fmt: v => fmt(v), sub: x => `K/D ${kd(x).toFixed(2)} · ${Math.round(hsPct(x))} % HS` },
      kd: { label: 'K/D', val: kd, min: true, fmt: v => v.toFixed(2), sub: kills },
      hs: { label: t('Headshots'), val: hsPct, min: true, fmt: v => `${Math.round(v)} %`, sub: x => t('{n} headshots', { n: fmt(x.headshots) }) }
    };
    if (ps.some(x => x.longest != null)) m.longest = { label: t('Plus long kill'), val: x => x.longest ?? 0, fmt: v => `${Math.round(v)} m`, sub: kills };
    if (ps.some(x => x.noscopes > 0)) m.noscopes = { label: t('Noscopes'), val: x => x.noscopes ?? 0, fmt: v => fmt(v), sub: kills };
    if (ps.some(x => x.quickscopes > 0)) m.quickscopes = { label: t('Quickscopes'), val: x => x.quickscopes ?? 0, fmt: v => fmt(v), sub: kills };
    if (ps.some(x => x.wallbangs > 0)) m.wallbangs = { label: t('Wallbangs'), val: x => x.wallbangs ?? 0, fmt: v => fmt(v), sub: kills };
    if (ps.some(x => x.shots >= 50)) m.acc = { label: t('Précision'), val: x => (x.shots >= 50 ? (100 * x.hits) / x.shots : -1), fmt: v => `${Math.round(v)} %`, sub: x => t('{n} tirs', { n: fmt(x.shots) }) };
    if (ps.some(x => x.dmg_done > 0 && x.rounds > 0)) m.dmg = { label: t('Dégâts / round'), val: x => (x.rounds ? (x.dmg_done ?? 0) / x.rounds : -1), min: true, fmt: v => fmt(Math.round(v)), sub: x => t('{n} rounds', { n: fmt(x.rounds) }) };
    if (ps.some(x => x.clutches > 0)) m.clutches = { label: t('Clutchs'), val: x => x.clutches ?? 0, fmt: v => fmt(v), sub: kills };
    if (ps.some(x => x.first_bloods > 0)) m.fb = { label: t('Premier sang'), val: x => x.first_bloods ?? 0, fmt: v => fmt(v), sub: kills };
    if (ps.some(x => x.rounds_played > 0)) m.win = { label: t('Victoires'), val: x => (x.rounds_played ? (100 * (x.rounds_won ?? 0)) / x.rounds_played : -1), min: true, fmt: v => `${Math.round(v)} %`, sub: x => t('{n} rounds', { n: fmt(x.rounds_played) }) };
    return m;
  });
  $effect(() => { if (!metrics[metric]) metric = 'kills'; });

  const ranked = $derived.by(() => {
    const m = metrics[metric] ?? metrics.kills;
    return (combat ?? [])
      .filter(x => (x.kills ?? 0) > 0 && (!m.min || x.kills >= minKills) && m.val(x) >= 0)
      .map(x => ({ ...x, v: m.val(x) }))
      .sort((a, b) => b.v - a.v || (b.kills ?? 0) - (a.kills ?? 0))
      .map((x, i) => ({ ...x, rank: i + 1 }));
  });
  const shown = $derived(ranked.slice(0, limit));
  const mine = $derived(me ? ranked.find(x => x.key === me.key) : null);
  const star = $derived((combat ?? []).find(x => (x.kills ?? 0) > 0) ?? null);

  // PC: every player of the period, filtered by name
  const tableRows = $derived.by(() => {
    const f = filter.trim().toLowerCase();
    return (combat ?? []).filter(x => !f || clean(x.name_raw).toLowerCase().includes(f));
  });
  const COLS = [
    { k: 'kills', label: t('Kills'), v: x => x.kills ?? 0, strong: true },
    { k: 'deaths', label: t('Morts'), v: x => x.deaths ?? 0 },
    { k: 'kd', label: 'K/D', v: kd, f: x => kd(x).toFixed(2), strong: true },
    { k: 'hs', label: 'HS', title: t('Headshots'), v: x => x.headshots ?? 0, f: x => `${x.headshots ?? 0} (${pct(x.headshots, x.kills)})` },
    { k: 'acc', label: t('Précision'), title: t('touches / tirs au sniper'), v: x => (x.shots ? x.hits / x.shots : null), f: x => (x.shots ? pct(x.hits, x.shots) : '—') },
    { k: 'dmg', label: t('Dégâts / round'), v: x => (x.rounds ? x.dmg_done / x.rounds : null), f: x => (x.rounds ? fmt(Math.round(x.dmg_done / x.rounds)) : '—') },
    { k: 'tk', label: t('Kills équipe'), v: x => x.teamkills ?? 0 },
    { k: 'sui', label: t('Suicides'), v: x => x.suicides ?? 0 },
    { k: 'ns', label: 'NS', title: t('Noscopes'), v: x => x.noscopes },
    { k: 'qs', label: 'QS', title: t('Quickscopes'), v: x => x.quickscopes },
    { k: 'wb', label: 'WB', title: t('Wallbangs'), v: x => x.wallbangs },
    { k: 'longest', label: t('Plus long'), v: x => x.longest, f: x => meters(x.longest) },
    { k: 'clutch', label: t('Clutchs'), title: t('rounds gagnés en étant le dernier en vie de son équipe'), v: x => x.clutches ?? 0 },
    { k: 'fb', label: t('Premier sang'), title: t('premier kill du round'), v: x => x.first_bloods ?? 0 },
    { k: 'win', label: t('Victoires'), title: t('rounds gagnés / rounds joués'), v: x => (x.rounds_played ? x.rounds_won / x.rounds_played : null), f: x => (x.rounds_played ? pct(x.rounds_won, x.rounds_played) : '—') },
    { k: 'weapon', label: t('Arme favorite'), text: true, v: x => (x.weapon ? weaponName(x.weapon) : null) }
  ];

  const recordCards = $derived.by(() => {
    if (!records) return [];
    const r = records;
    /** @type {{ k: string, label: string, value: string, key: string, name: string, map?: string, ts?: number }[]} */
    const out = [];
    const top = (k, label, x) => { if (x) out.push({ k, label: t(label), value: t('{n} kills', { n: fmt(x.kills) }), key: x.key, name: x.name_raw }); };
    const rec = (k, label, f) => { if (r[k]) out.push({ k, label: t(label), value: f(r[k].Value), key: r[k].Key, name: r[k].Name, map: r[k].Map, ts: r[k].TS }); };
    top('day', 'Joueur du jour', tops.day);
    top('week', 'Joueur de la semaine', tops.week);
    rec('streak', 'Meilleure série', v => t('{n} d\'affilée', { n: v }));
    rec('round_kills', 'Kills en un round', v => fmt(v));
    rec('match_kills', 'Kills en un match', v => fmt(v));
    rec('match_headshots', 'Headshots en un match', v => fmt(v));
    rec('longest', 'Plus long kill', v => `${v} m`);
    rec('longest_noscope', 'Plus long noscope', v => `${v} m`);
    return out;
  });

  const BADGES = {
    killer: ['Machine à tuer', v => t('{n} kills', { n: fmt(v) })],
    kd: ['Meilleur K/D', v => Number(v).toFixed(2)], headshot: ['Roi du headshot', v => `${v} %`], accuracy: ['Meilleure précision', v => `${v} %`],
    active: ['Plus assidu', v => `${Math.round(v / 3600)} h`], loyal: ['Plus fidèle', v => t('{n} jours', { n: v })], noscope: ['Noscopes', v => fmt(v)],
    quickscope: ['Quickscopes', v => fmt(v)], wallbang: ['Wallbangs', v => fmt(v)], longest: ['Plus long tir', v => `${v} m`], clutch: ['Clutchs', v => fmt(v)],
    winner: ['Meilleur gagnant', v => t('{n} % de rounds gagnés', { n: v })], chatty: ['Plus bavard', v => t('{n} messages', { n: fmt(v) })]
  };
  const badgeRows = $derived(badges.filter(b => BADGES[b.badge]).map(b => ({ ...b, label: t(BADGES[b.badge][0]), value: BADGES[b.badge][1](b.value) })));
</script>

<h1 class="title">{t('Classements')}</h1>

<Seg label={t('Période')} value={period} onchange={v => (period = v)} items={[['7d', t('7 jours')], ['30d', t('30 jours')], ['90d', t('90 jours')]]} />

{#snippet who(x)}<a class="tn" href={href('joueur', x.key)}><CodName raw={x.name_raw} /></a>{/snippet}

{#if combat === null}
  <p class="muted pad">{t('Chargement…')}</p>
{:else}
  <div class="stack top">
    {#if star}
      <a class="card star" href={href('joueur', star.key)}>
        <span class="ic"><Icon name="trophy" size={26} /></span>
        <span class="txt">
          <span class="label">{t('Joueur {p}', { p: periodWord })}</span>
          <span class="nm"><CodName raw={star.name_raw} /></span>
          <span class="line">{fmt(star.kills)} kills · K/D {kd(star).toFixed(2)} · {Math.round(hsPct(star))} % HS</span>
        </span>
      </a>
    {/if}

    <section class="card box phone" aria-label={t('Top joueurs')}>
      <div class="hd"><h2 class="label">{t('Top joueurs')}</h2></div>
      <div class="chips" role="group" aria-label={t('Classer par')}>
        {#each Object.entries(metrics) as [k, m] (k)}
          <button aria-pressed={metric === k} onclick={() => { metric = k; limit = 10; }}>{m.label}</button>
        {/each}
      </div>
      {#if metrics[metric]?.min}<p class="note">{t('Joueurs avec au moins {n} kills sur la période.', { n: minKills })}</p>{/if}
      {#each shown as x (x.key)}
        <a class="row" class:mine={x.key === me?.key} href={href('joueur', x.key)}>
          <span class="rk" class:podium={x.rank <= 3}>{x.rank}</span>
          <span class="mid"><span class="pn"><CodName raw={x.name_raw} /></span><span class="sub">{metrics[metric].sub(x)}</span></span>
          <span class="val">{metrics[metric].fmt(x.v)}</span>
        </a>
      {:else}
        <p class="muted empty">{t('Pas encore de kills sur cette période.')}</p>
      {/each}
      {#if mine && mine.rank > shown.length}
        <a class="row mine sep" href={href('joueur', mine.key)}>
          <span class="rk">{mine.rank}</span>
          <span class="mid"><span class="pn">{t('Ta place')} · <CodName raw={mine.name_raw} /></span><span class="sub">{metrics[metric].sub(mine)}</span></span>
          <span class="val">{metrics[metric].fmt(mine.v)}</span>
        </a>
      {/if}
      {#if ranked.length > 10}
        {#if ranked.length > limit}<button class="btn ghost more" onclick={() => (limit += 50)}>{t('Afficher plus ({n} sur {total})', { n: limit, total: ranked.length })}</button>
        {:else if limit > 10}<button class="btn ghost more" onclick={() => (limit = 10)}>{t('Réduire')}</button>{/if}
      {/if}
    </section>

    <section class="card box pc" aria-label={t('Classement combat')}>
      <div class="hd">
        <h2 class="label">{t('Classement combat')}</h2>
        <span class="muted small">{t('{n} joueurs', { n: fmt(tableRows.length) })}</span>
        <input class="flt" type="search" bind:value={filter} placeholder={t('Filtrer…')} aria-label={t('Filtrer les joueurs')} autocomplete="off" spellcheck="false" />
      </div>
      <DataTable rows={tableRows} cols={COLS} name={who} sort="kills" keyOf={x => x.key} mine={me?.key ?? ''} label={t('Classement combat')} empty={t('Aucun kill sur la période')} />
    </section>
  </div>

  <div class="grid">
    <div class="stack">
      {#if challenges?.challenges?.length}
        <Challenges data={challenges} sid={id} {owner} />
      {/if}

      {#if badgeRows.length}
        <section class="card box" aria-label={t('Distinctions')}>
          <h2 class="label">{t('Distinctions {p}', { p: periodWord })}</h2>
          {#each badgeRows as b (b.badge)}
            <a class="badge" href={href('joueur', b.key)}>
              <span class="bl">{b.label}</span>
              <span class="bn"><CodName raw={b.name_raw} /></span>
              <span class="bv">{b.value}</span>
            </a>
          {/each}
        </section>
      {/if}
    </div>

    <div class="stack">
      {#if recordCards.length}
        <section aria-label={t('Records')}>
          <h2 class="label sec">{t('Records')}</h2>
          <div class="records">
            {#each recordCards as rc (rc.k)}
              <a class="card rec" class:hl={rc.k === 'day' || rc.k === 'week'} href={href('joueur', rc.key)}>
                <span class="rl">{rc.label}</span>
                <b>{rc.value}</b>
                <span class="who"><CodName raw={rc.name} /></span>
                {#if rc.map || rc.ts}<span class="when muted">{rc.map ? mapName(rc.map) : ''}{#if rc.map && rc.ts}{' · '}{/if}{rc.ts ? day(rc.ts) : ''}</span>{/if}
              </a>
            {/each}
          </div>
        </section>
      {/if}

      <div class="card links">
        <a href={href('matchs')}><span><b>{t('Matchs récents')}</b><small>{t('tableaux des scores, rounds, MVP')}</small></span><Icon name="chevronRight" /></a>
        <a href={href('duel')}><span><b>{t('Duel')}</b><small>{t('qui a tué qui entre deux joueurs')}</small></span><Icon name="chevronRight" /></a>
        <a href={href('stats')}><span><b>{t('Stats avancées')}</b><small>{t('affluence, maps, votes, fidélité, armes')}</small></span><Icon name="chevronRight" /></a>
      </div>
    </div>
  </div>
{/if}

<style>
  .title { margin-bottom: 14px; }
  .pad { margin-top: 14px; }
  .top { margin-top: 14px; }
  .grid { display: grid; gap: 12px; margin-top: 12px; }
  .stack { display: flex; flex-direction: column; gap: 12px; min-width: 0; }
  .star { display: flex; align-items: center; gap: 14px; padding: 16px; text-decoration: none; color: var(--text); border-color: var(--accent); background: linear-gradient(135deg, var(--accent-soft), transparent 70%), var(--surface); }
  .star .ic { width: 52px; height: 52px; flex: none; border-radius: 16px; background: var(--accent); color: var(--on-accent); display: flex; align-items: center; justify-content: center; }
  .txt { min-width: 0; display: flex; flex-direction: column; }
  .star .label { color: var(--accent); }
  .nm { font-family: var(--font-display); font-size: 24px; font-weight: 700; display: flex; min-width: 0; }
  .line { font-size: 13px; color: var(--text-2); }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; }
  .hd { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; }
  .small { font-size: 12px; margin: 0; }
  .chips { display: flex; gap: 6px; overflow-x: auto; padding-bottom: 2px; scrollbar-width: none; }
  .chips button { flex: none; height: 34px; padding: 0 12px; border-radius: 17px; border: 1px solid var(--line); background: transparent; color: var(--text-2); font-weight: 700; font-size: 13px; }
  .chips button[aria-pressed='true'] { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .note { margin: 0; font-size: 12px; color: var(--muted); }
  .row { display: flex; align-items: center; gap: 12px; min-height: 56px; padding: 6px 4px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); }
  .row.mine { background: var(--accent-soft); margin: 0 -8px; padding: 6px 12px; border-radius: 10px; }
  .row.sep { margin-top: 6px; }
  .rk { width: 30px; height: 30px; flex: none; border-radius: 9px; background: var(--surface-2); display: flex; align-items: center; justify-content: center; font-weight: 800; font-size: 13px; color: var(--text-2); }
  .rk.podium { background: var(--accent); color: var(--on-accent); }
  .mid { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .pn { display: flex; gap: 4px; min-width: 0; font-weight: 700; white-space: nowrap; overflow: hidden; }
  .sub { font-size: 12px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .val { font-family: var(--font-display); font-size: 18px; font-weight: 700; font-variant-numeric: tabular-nums; }
  .empty { margin: 8px 0 0; }
  .more { align-self: center; min-height: 40px; color: var(--accent); }
  .pc { display: none; }
  .flt { margin-left: auto; width: 220px; height: 36px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text); padding: 0 10px; font-size: 16px; }
  .tn { display: flex; min-width: 0; color: var(--text); text-decoration: none; }
  .tn:hover { text-decoration: underline; }
  .sec { display: block; margin: 4px 2px 8px; }
  .records { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px; }
  .rec { padding: 12px 14px; display: flex; flex-direction: column; gap: 2px; text-decoration: none; color: var(--text); min-width: 0; }
  .rec.hl { border-color: var(--accent); }
  .rl { font-size: 12px; color: var(--muted); font-weight: 700; }
  .rec b { font-family: var(--font-display); font-size: 22px; }
  .who { display: flex; min-width: 0; font-size: 13px; font-weight: 700; white-space: nowrap; overflow: hidden; }
  .when { font-size: 12px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .badge { display: grid; grid-template-columns: minmax(0, 1.1fr) minmax(0, 1fr) auto; gap: 8px; align-items: center; min-height: 44px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); font-size: 14px; }
  .bl { color: var(--text-2); }
  .bn { display: flex; min-width: 0; font-weight: 700; overflow: hidden; white-space: nowrap; }
  .bv { font-weight: 800; font-variant-numeric: tabular-nums; text-align: right; }
  .links { overflow: hidden; }
  .links a { display: flex; align-items: center; gap: 10px; padding: 14px 16px; text-decoration: none; color: var(--text); border-top: 1px solid var(--line-soft); }
  .links a:first-child { border-top: 0; }
  .links span { flex: 1; display: flex; flex-direction: column; }
  .links small { color: var(--muted); font-size: 13px; }
  @media (min-width: 960px) {
    .phone { display: none; }
    .pc { display: flex; }
    .hd .small { margin-right: auto; }
    .flt { margin-left: 0; }
    .grid { grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); align-items: start; }
    .records { grid-template-columns: repeat(2, minmax(0, 1fr)); }
  }
</style>
