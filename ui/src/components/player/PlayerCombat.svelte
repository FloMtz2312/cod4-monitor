<script>
  // Combat of a profile (all time): weapons (kills, headshots), body zones hit, nemeses and favourite victims,
  // then the figures of the sniper mod and of the logs.
  import CodName from '../CodName.svelte';
  import { href } from '../../lib/router.svelte.js';
  import { weaponName, fmt } from '../../lib/format.js';
  import { t } from '../../lib/i18n.svelte.js';

  /** @type {{ p: any }} */
  let { p } = $props();

  const one = v => (Array.isArray(v) ? v[0] : v) ?? {};
  const sniper = $derived(one(p.sniper));
  const combat = $derived(one(p.combat));
  const acc = $derived(one(p.accuracy));
  const best = $derived(one(p.best_score).best);
  const pct = (n, total) => (total ? Math.round((100 * n) / total) : 0);

  // weapons: same display name merged (attachments), all of them, with their headshot rate
  const weapons = $derived.by(() => {
    const by = new Map();
    for (const w of p.weapons ?? []) {
      const n = weaponName(w.weapon) || t('Autre');
      const x = by.get(n) ?? { n: 0, hs: 0 };
      x.n += w.kills ?? 0;
      x.hs += w.headshots ?? 0;
      by.set(n, x);
    }
    const total = [...by.values()].reduce((a, b) => a + b.n, 0);
    return [...by].sort((a, b) => b[1].n - a[1].n).map(([label, x]) => ({ label, n: x.n, pct: pct(x.n, total), hs: pct(x.hs, x.n) }));
  });

  // every zone of the game's hit locations
  const ZONE = {
    head: 'Tête', helmet: 'Casque', neck: 'Cou', torso_upper: 'Haut du torse', torso_lower: 'Bas du torse',
    right_arm_upper: 'Bras droit', left_arm_upper: 'Bras gauche', right_arm_lower: 'Avant-bras droit', left_arm_lower: 'Avant-bras gauche',
    right_hand: 'Main droite', left_hand: 'Main gauche', right_leg_upper: 'Cuisse droite', left_leg_upper: 'Cuisse gauche',
    right_leg_lower: 'Jambe droite', left_leg_lower: 'Jambe gauche', right_foot: 'Pied droit', left_foot: 'Pied gauche'
  };
  const zones = $derived.by(() => {
    const list = (p.hitlocs ?? []).filter(h => h.hitloc && h.hitloc !== 'none' && h.kills > 0);
    const total = list.reduce((a, h) => a + h.kills, 0);
    return list.map(h => ({ label: ZONE[h.hitloc] ? t(ZONE[h.hitloc]) : h.hitloc, n: h.kills, pct: pct(h.kills, total) }));
  });
  const nemeses = $derived((p.nemesis ?? []).slice(0, 5));
  const victims = $derived((p.victims ?? []).slice(0, 5));

  const tiles = $derived([
    sniper.noscopes != null && { label: t('Noscopes'), v: fmt(sniper.noscopes) },
    sniper.quickscopes != null && { label: t('Quickscopes'), v: fmt(sniper.quickscopes) },
    sniper.wallbangs != null && { label: t('Wallbangs'), v: fmt(sniper.wallbangs) },
    sniper.avg_distance != null && { label: t('Distance moy.'), v: `${Math.round(sniper.avg_distance)} m` },
    sniper.clutches != null && { label: t('Clutchs'), v: fmt(sniper.clutches) },
    (sniper.plants != null || sniper.defuses != null) && { label: t('Bombes posées / désamorcées'), v: `${sniper.plants ?? 0} / ${sniper.defuses ?? 0}` },
    sniper.rounds_played > 0 && { label: t('Rounds gagnés'), v: `${pct(sniper.rounds_won ?? 0, sniper.rounds_played)} %` },
    acc.shots > 0 && { label: t('Précision'), v: `${pct(acc.hits, acc.shots)} %` },
    sniper.points > 0 && { label: t('Points'), v: fmt(sniper.points) },
    best != null && { label: t('Meilleur score'), v: fmt(best) },
    { label: t('Morts'), v: fmt(combat.deaths ?? 0) },
    combat.teamkills > 0 && { label: t('Tirs alliés mortels'), v: fmt(combat.teamkills) },
    combat.suicides > 0 && { label: t('Suicides'), v: fmt(combat.suicides) }
  ].filter(Boolean));
</script>

<div class="stack">
  {#if weapons.length || zones.length}
    <section class="card box" aria-label={t('Armes et zones touchées')}>
      {#if weapons.length}
        <h2 class="label">{t('Armes')}</h2>
        {#each weapons as w (w.label)}
          <div class="bar"><div class="row"><span>{w.label}</span><span class="muted">{t('{n} kills · {p} %', { n: fmt(w.n), p: w.pct })}{' · '}{t('{p} % HS', { p: w.hs })}</span></div><div class="track"><div style:width={`${w.pct}%`}></div></div></div>
        {/each}
      {/if}
      {#if zones.length}
        <h2 class="label zh">{t('Zones touchées')}</h2>
        {#each zones as z (z.label)}
          <div class="bar zone"><div class="row"><span>{z.label}</span><span class="muted">{fmt(z.n)} · {z.pct} %</span></div><div class="track"><div style:width={`${z.pct}%`}></div></div></div>
        {/each}
      {/if}
    </section>
  {/if}

  {#if nemeses.length || victims.length}
    <div class="duo">
      <section class="card box" aria-label={t('Ses némésis')}>
        <h2 class="label">{t('Ses némésis')}</h2>
        {#each nemeses as x, i (x.key)}
          <a class="who" href={href('joueur', x.key)}><span class="rk">{i + 1}</span><span class="nm"><CodName raw={x.name_raw || x.key} /></span><span class="muted">{t('l\'a tué {n} fois', { n: x.kills })}</span></a>
        {:else}<p class="muted">—</p>{/each}
      </section>
      <section class="card box" aria-label={t('Ses victimes favorites')}>
        <h2 class="label">{t('Ses victimes favorites')}</h2>
        {#each victims as x, i (x.key)}
          <a class="who" href={href('joueur', x.key)}><span class="rk">{i + 1}</span><span class="nm"><CodName raw={x.name_raw || x.key} /></span><span class="muted">{t('tué {n} fois', { n: x.kills })}</span></a>
        {:else}<p class="muted">—</p>{/each}
      </section>
    </div>
  {/if}

  <div class="tiles">
    {#each tiles as k (k.label)}<div class="card"><b>{k.v}</b><span>{k.label}</span></div>{/each}
  </div>
</div>

<style>
  .stack { display: flex; flex-direction: column; gap: 12px; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
  .zh { margin-top: 8px; }
  .row { display: flex; justify-content: space-between; gap: 8px; font-size: 14px; font-weight: 700; margin-bottom: 6px; }
  .row .muted { font-weight: 600; font-size: 13px; }
  .track { height: 8px; border-radius: 4px; background: var(--surface-2); overflow: hidden; }
  .track div { height: 100%; border-radius: 4px; background: var(--accent); }
  .zone .track div { background: var(--allies); }
  .duo { display: grid; grid-template-columns: minmax(0, 1fr); gap: 12px; }
  .box p { margin: 0; }
  .who { display: flex; align-items: center; gap: 10px; min-height: 36px; text-decoration: none; color: var(--text); font-size: 14px; }
  .who .rk { width: 22px; height: 22px; flex: none; border-radius: 7px; background: var(--surface-2); color: var(--text-2); font-size: 12px; font-weight: 800; display: flex; align-items: center; justify-content: center; }
  .who .nm { flex: 1; min-width: 0; display: flex; font-weight: 700; }
  .who .muted { font-size: 12px; white-space: nowrap; }
  .tiles { display: grid; grid-template-columns: repeat(3, minmax(0, 1fr)); gap: 8px; }
  .tiles .card { padding: 12px; display: flex; flex-direction: column; gap: 2px; min-width: 0; }
  .tiles b { font-family: var(--font-display); font-size: 19px; font-variant-numeric: tabular-nums; }
  .tiles span { font-size: 12px; color: var(--muted); }
  @media (min-width: 720px) { .duo { grid-template-columns: 1fr 1fr; } }
  @media (min-width: 720px) { .tiles { grid-template-columns: repeat(4, minmax(0, 1fr)); } }
</style>
