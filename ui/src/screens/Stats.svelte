<script>
  // Stats avancées, in tabs (#/stats/<tab>): affluence (players over time, map changes, busy hours),
  // maps (ranking with a verdict, retention, rounds, votes), loyalty (new / returning players) and server-wide combat.
  import Icon from '../components/Icon.svelte';
  import Seg from '../components/Seg.svelte';
  import StatsAffluence from '../components/stats/StatsAffluence.svelte';
  import StatsMaps from '../components/stats/StatsMaps.svelte';
  import StatsLoyalty from '../components/stats/StatsLoyalty.svelte';
  import StatsCombat from '../components/stats/StatsCombat.svelte';
  import '../components/stats/stats.css';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { back, route, replace } from '../lib/router.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const TABS = ['affluence', 'maps', 'fidelite', 'combat'];
  const id = $derived(app.serverId);
  const srv = $derived(currentServer());
  const mod = $derived(!!srv?.mod);
  const tab = $derived(TABS.includes(route.params[0]) ? route.params[0] : 'affluence');
  let period = $state('30d');
</script>

<button class="back" onclick={() => back('classements')}><Icon name="back" />{t('Retour')}</button>
<h1 class="title">{t('Stats avancées')}</h1>
<div class="bar">
  <Seg label={t('Rubrique')} value={tab} onchange={v => replace('stats', v)}
    items={[['affluence', t('Affluence')], ['maps', t('Maps')], ['fidelite', t('Fidélité')], ['combat', t('Combat')]]} />
  <Seg label={t('Période')} value={period} onchange={v => (period = v)} items={[['7d', t('7 jours')], ['30d', t('30 jours')], ['90d', t('90 jours')]]} />
</div>

{#if id}
  {#key id}
    {#if tab === 'affluence'}
      <StatsAffluence {id} {period} slots={srv?.max_players ?? null} />
    {:else if tab === 'maps'}
      <StatsMaps {id} {period} {mod} />
    {:else if tab === 'fidelite'}
      <StatsLoyalty {id} {period} />
    {:else}
      <StatsCombat {id} {period} {mod} />
    {/if}
  {/key}
{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 4px; height: 40px; padding: 0 10px 0 4px; margin: -4px 0 8px -4px; border: 0; background: transparent; color: var(--text-2); font-weight: 700; }
  .title { margin-bottom: 14px; }
  .bar { display: grid; gap: 8px; }
  @media (min-width: 960px) {
    .bar { grid-template-columns: minmax(0, 3fr) minmax(0, 2fr); }
  }
</style>
