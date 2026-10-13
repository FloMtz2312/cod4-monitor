<script>
  // Combat of the whole server: how people kill (headshots, noscopes…), the weapons and the body zones hit.
  import { get } from '../../lib/api.js';
  import { fmt, weaponName } from '../../lib/format.js';
  import { t } from '../../lib/i18n.svelte.js';
  import { pct } from './util.js';

  /** @type {{ id: number, period: string, mod: boolean }} */
  let { id, period, mod } = $props();

  let c = $state(null);
  $effect(() => {
    const sid = id, p = period;
    let alive = true;
    c = null;
    get(`/api/servers/${sid}/combat?range=${p}`).then(r => { if (alive) c = r; }).catch(() => { if (alive) c = {}; });
    return () => { alive = false; };
  });

  const players = $derived(c?.players ?? []);
  const sum = k => players.reduce((a, x) => a + (x[k] ?? 0), 0);
  const kills = $derived(sum('kills'));
  // average kill distance, weighted by the kills of each player
  const dist = $derived.by(() => {
    const dk = players.filter(x => x.avg_distance != null);
    const n = dk.reduce((a, x) => a + x.kills, 0);
    return n ? dk.reduce((a, x) => a + x.avg_distance * x.kills, 0) / n : null;
  });
  const style = $derived(kills ? [
    { v: pct(sum('headshots'), kills), label: t('headshots') },
    mod && { v: pct(sum('noscopes'), kills), label: t('noscopes') },
    mod && { v: pct(sum('quickscopes'), kills), label: t('quickscopes') },
    mod && { v: pct(sum('wallbangs'), kills), label: t('wallbangs') },
    mod && dist != null && { v: `${Math.round(dist)} m`, label: t('distance moyenne') }
  ].filter(Boolean) : []);

  const COLORS = ['#F5A524', '#5EA0FF', '#B18CFF', '#3DD68C', '#8D95A6'];
  const weapons = $derived((c?.weapons ?? []).filter(w => w.weapon));
  const wTotal = $derived(weapons.reduce((a, w) => a + w.kills, 0));
  const wSplit = $derived.by(() => {
    const top = weapons.slice(0, 4).map(w => ({ label: weaponName(w.weapon), kills: w.kills }));
    if (weapons.length > 4) top.push({ label: t('autres'), kills: weapons.slice(4).reduce((a, w) => a + w.kills, 0) });
    return top;
  });

  const ZONES = {
    head: 'Tête', helmet: 'Casque', neck: 'Cou', torso_upper: 'Haut du torse', torso_lower: 'Bas du torse',
    right_arm_upper: 'Bras droit', right_arm_lower: 'Avant-bras droit', right_hand: 'Main droite',
    left_arm_upper: 'Bras gauche', left_arm_lower: 'Avant-bras gauche', left_hand: 'Main gauche',
    right_leg_upper: 'Cuisse droite', right_leg_lower: 'Jambe droite', right_foot: 'Pied droit',
    left_leg_upper: 'Cuisse gauche', left_leg_lower: 'Jambe gauche', left_foot: 'Pied gauche', none: 'Aucune zone'
  };
  const hitlocs = $derived((c?.hitlocs ?? []).filter(h => h.hitloc != null));
  const hTotal = $derived(hitlocs.reduce((a, h) => a + h.kills, 0));
  const hMax = $derived(Math.max(1, ...hitlocs.map(h => h.kills)));
</script>

{#if c === null}
  <p class="muted pad">{t('Chargement…')}</p>
{:else if !kills && !weapons.length}
  <p class="muted pad">{t('Aucun kill sur la période.')}</p>
{:else}
  <div class="s-grid">
    <section class="card s-box s-wide" aria-label={t('Style de jeu du serveur')}>
      <div class="s-hd"><h2 class="label">{t('Style de jeu du serveur')}</h2><span class="muted s-small">{t('{n} kills sur la période', { n: fmt(kills) })}</span></div>
      <div class="s-tiles">
        {#each style as s (s.label)}<div><b>{s.v}</b><span>{s.label}</span></div>{/each}
      </div>
    </section>

    <section class="card s-box" aria-label={t('Armes')}>
      <h2 class="label">{t('Armes')}</h2>
      {#if wTotal}
        <div class="split" role="img" aria-label={wSplit.map(w => `${w.label} ${pct(w.kills, wTotal)}`).join(', ')}>
          {#each wSplit as w, i (w.label)}<span style:flex={w.kills} style:background={COLORS[i]} title={`${w.label} : ${w.kills}`}></span>{/each}
        </div>
        <div class="s-legend">{#each wSplit as w, i (w.label)}<span><i style:background={COLORS[i]}></i><b>{w.label}</b> {pct(w.kills, wTotal)}</span>{/each}</div>
        <table class="s-table">
          <thead><tr><th>{t('Arme')}</th><th class="n">{t('Kills')}</th><th class="n">{t('Headshots')}</th></tr></thead>
          <tbody>
            {#each weapons as w (w.weapon)}
              <tr><td><b>{weaponName(w.weapon)}</b> <span class="muted id">{w.weapon}</span></td><td class="n">{fmt(w.kills)}</td><td class="n">{pct(w.headshots, w.kills)}</td></tr>
            {/each}
          </tbody>
        </table>
      {:else}
        <p class="muted">—</p>
      {/if}
    </section>

    <section class="card s-box" aria-label={t('Zones touchées')}>
      <h2 class="label">{t('Zones touchées')}</h2>
      {#each hitlocs as h (h.hitloc)}
        <div class="hz">
          <div class="mt"><span>{t(ZONES[h.hitloc] ?? h.hitloc)}</span><span class="muted">{fmt(h.kills)} · {pct(h.kills, hTotal)}</span></div>
          <div class="s-track"><div style:width="{(100 * h.kills) / hMax}%"></div></div>
        </div>
      {:else}
        <p class="muted">—</p>
      {/each}
    </section>
  </div>
{/if}

<style>
  .pad { margin-top: 14px; }
  .split { display: flex; height: 12px; border-radius: 6px; overflow: hidden; gap: 2px; }
  .id { font-size: 11px; }
  .hz { display: flex; flex-direction: column; gap: 5px; }
  .mt { display: flex; justify-content: space-between; gap: 8px; font-size: 14px; font-weight: 700; }
  .mt .muted { font-weight: 600; font-size: 13px; }
</style>
