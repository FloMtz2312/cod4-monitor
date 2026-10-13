<script>
  // Community challenges of the week (UTC Monday to Monday): goals, top contributors, best days ever, and for the
  // owner the goals of the next weeks.
  import Icon from './Icon.svelte';
  import CodName from './CodName.svelte';
  import { api } from '../lib/api.js';
  import { href } from '../lib/router.svelte.js';
  import { fmt, day } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ data: any, sid: number, owner: boolean }} */
  let { data, sid, owner } = $props();

  const GOALS = { kills: 'Kills tous ensemble', headshots: 'Headshots tous ensemble', objectives: 'Bombes posées ou désamorcées' };
  const MAX = { kills: 10000000, headshots: 10000000, objectives: 1000000 };

  let editing = $state(false);
  let saving = $state(false);
  let status = $state('');
  /** @type {Record<string, number>} */
  let goals = $state({});

  function edit() {
    goals = Object.fromEntries(data.challenges.map(c => [c.metric, c.target]));
    status = '';
    editing = true;
  }

  async function save(e) {
    e.preventDefault();
    saving = true;
    status = '';
    try {
      await api('PUT', `/api/servers/${sid}/challenges`, { kills: Number(goals.kills), headshots: Number(goals.headshots), objectives: Number(goals.objectives) });
      status = t('Enregistré pour la prochaine semaine.');
      editing = false;
    } catch (err) {
      status = err.message;
    } finally {
      saving = false;
    }
  }
</script>

<section class="card box" aria-label={t('Défis de la semaine')}>
  <div class="hd">
    <h2 class="label">{t('Défis de la semaine')}</h2>
    <span class="muted small">{t('du {from} au {to}', { from: day(data.from, true), to: day(data.to - 86400, true) })}</span>
  </div>
  {#each data.challenges as g (g.metric)}
    <div class="goal">
      <div class="gl"><span>{t(GOALS[g.metric] ?? g.metric)}{#if g.completed}{' '}<Icon name="check" size={16} />{/if}</span><span class="muted">{fmt(g.progress)} / {fmt(g.target)}</span></div>
      <div class="track"><div class:done={g.completed} style:width={`${Math.min(100, (100 * g.progress) / Math.max(1, g.target))}%`}></div></div>
    </div>
  {/each}
  <p class="muted small">{t('Remise à zéro le lundi à 00:00 UTC')}{#if data.contributors}{' · '}{t('{n} joueurs y ont contribué cette semaine.', { n: data.contributors })}{/if}</p>

  <div class="two">
    <div>
      <h3 class="sub">{t('Top de la semaine')}</h3>
      {#each data.leaders ?? [] as p, i (p.key)}
        <a class="li" href={href('joueur', p.key)}><span class="i">{i + 1}</span><span class="pn"><CodName raw={p.name_raw} /></span><b>{fmt(p.kills)}</b></a>
      {:else}
        <p class="muted small">{t('Les premiers kills lanceront le classement.')}</p>
      {/each}
    </div>
    <div>
      <h3 class="sub">{t('Meilleures journées (UTC)')}</h3>
      {#each data.daily_records ?? [] as r (r.ts)}
        <div class="li"><span class="pn">{day(r.ts, true)}</span><span class="muted">{r.headshots} HS</span><b>{fmt(r.kills)}</b></div>
      {:else}
        <p class="muted small">{t('Aucun record enregistré.')}</p>
      {/each}
    </div>
  </div>

  {#if owner}
    {#if editing}
      <form class="ed" onsubmit={save}>
        <h3 class="sub">{t('Objectifs des prochaines semaines')}</h3>
        {#each data.challenges as g (g.metric)}
          <label><span>{t(GOALS[g.metric] ?? g.metric)}</span><input type="number" min="1" max={MAX[g.metric]} required bind:value={goals[g.metric]} /></label>
        {/each}
        <p class="muted small">{t('Les objectifs en cours restent fixes jusqu\'à la fin de la semaine.')}</p>
        <div class="acts">
          <button type="button" class="btn ghost" onclick={() => (editing = false)}>{t('Annuler')}</button>
          <button class="btn primary" disabled={saving}>{t('Enregistrer')}</button>
        </div>
      </form>
    {:else}
      <button class="btn ghost edit" onclick={edit}><Icon name="settings" size={16} />{t('Objectifs des prochaines semaines')}</button>
    {/if}
    {#if status}<p class="small status" aria-live="polite">{status}</p>{/if}
  {/if}
</section>

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; }
  .hd { display: flex; align-items: baseline; justify-content: space-between; gap: 8px; flex-wrap: wrap; }
  .small { font-size: 12px; margin: 0; }
  .goal { display: flex; flex-direction: column; gap: 6px; }
  .gl { display: flex; justify-content: space-between; gap: 8px; font-size: 14px; font-weight: 700; }
  .gl span:first-child { display: inline-flex; align-items: center; gap: 4px; color: var(--text); }
  .gl .muted { font-weight: 600; font-variant-numeric: tabular-nums; }
  .track { height: 8px; border-radius: 4px; background: var(--surface-2); overflow: hidden; }
  .track div { height: 100%; border-radius: 4px; background: var(--accent); }
  .track div.done { background: var(--ok); }
  .two { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 14px; margin-top: 4px; }
  .sub { margin: 4px 0 2px; font-size: 12px; font-weight: 800; color: var(--text-2); }
  .li { display: flex; align-items: center; gap: 6px; min-height: 32px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); font-size: 13px; }
  .li .i { width: 16px; color: var(--muted); font-weight: 800; font-size: 12px; }
  .li .pn { flex: 1; min-width: 0; display: flex; overflow: hidden; white-space: nowrap; font-weight: 700; }
  .li .muted { font-size: 12px; }
  .li b { font-variant-numeric: tabular-nums; }
  .edit { align-self: flex-start; min-height: 40px; padding: 0; color: var(--accent); gap: 6px; }
  .ed { display: flex; flex-direction: column; gap: 8px; border-top: 1px solid var(--line-soft); padding-top: 8px; }
  .ed label { display: flex; align-items: center; justify-content: space-between; gap: 10px; font-size: 14px; }
  .ed input { width: 130px; height: 40px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text); padding: 0 10px; font-size: 16px; text-align: right; }
  .acts { display: flex; gap: 8px; justify-content: flex-end; }
  .status { color: var(--text-2); }
</style>
