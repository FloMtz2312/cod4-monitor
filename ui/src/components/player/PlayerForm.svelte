<script>
  // Recent form: the last 7 days against the 7 days before, a comparison with another player over the same days,
  // and the last 10 rounds played (each one opens its match).
  import { get } from '../../lib/api.js';
  import { href } from '../../lib/router.svelte.js';
  import { mapName } from '../../lib/state.svelte.js';
  import { clean, fmt } from '../../lib/format.js';
  import { dt } from '../../lib/dates.js';
  import { t } from '../../lib/i18n.svelte.js';

  /** @type {{ form: any, playerKey: string, serverId: number }} */
  let { form, playerKey, serverId } = $props();

  /** @type {{key: string, name: string}[]} */
  let others = $state([]);
  let other = $state('');
  let cmp = $state(null);
  let cmpMsg = $state('');

  $effect(() => {
    const sid = serverId, k = playerKey;
    others = [];
    other = '';
    get(`/api/servers/${sid}/players?range=30d`).then(r => {
      others = (r.players ?? []).filter(p => p.key && p.key !== k)
        .map(p => ({ key: p.key, name: clean(p.name_raw || p.name_clean || p.key) }))
        .sort((a, b) => a.name.localeCompare(b.name));
    }).catch(() => {});
  });

  let seq = 0;
  $effect(() => {
    const o = other, sid = serverId, k = playerKey;
    cmp = null;
    cmpMsg = '';
    if (!o) return;
    const n = ++seq;
    cmpMsg = t('Chargement…');
    get(`/api/servers/${sid}/form?key=${encodeURIComponent(k)}&compare=${encodeURIComponent(o)}`)
      .then(r => { if (n === seq) { cmp = r.comparison ?? null; cmpMsg = cmp ? '' : t('Comparaison indisponible'); } })
      .catch(e => { if (n === seq) cmpMsg = e.message; });
  });

  const cur = $derived(form?.current);
  const prev = $derived(form?.previous);
  const delta = $derived(form?.sample_sufficient ? cur.kd - prev.kd : null);
  const otherName = $derived(others.find(o => o.key === other)?.name ?? '');
  const acc = f => (f.accuracy == null ? '—' : `${f.accuracy.toFixed(1)} %`);
  const cols = $derived([
    { label: t('Cette période'), f: cur },
    { label: t('Période précédente'), f: prev },
    ...(cmp ? [{ label: otherName, f: cmp, other: true }] : [])
  ]);
</script>

{#if form}
  <section class="card box" aria-label={t('Forme récente')}>
    <h2 class="label">{t('Forme récente')}</h2>
    <p class="muted small">{t('7 derniers jours comparés aux 7 jours précédents.')}</p>
    {#if delta != null}
      <p class="trend">{t('K/D :')} <b class:up={delta >= 0}>{delta >= 0 ? '+' : ''}{delta.toFixed(2)}</b> {t('par rapport à la période précédente')}</p>
    {:else}
      <p class="muted small">{t('Données encore limitées : au moins 20 kills + morts par période pour interpréter la tendance.')}</p>
    {/if}

    <div class="table" role="table" aria-label={t('Forme récente')}>
      <div class="tr head" role="row">
        <span role="columnheader"></span>
        <span role="columnheader">{t('Kills / morts')}</span><span role="columnheader">K/D</span><span role="columnheader">{t('Précision')}</span>
      </div>
      {#each cols as c (c.label)}
        <div class="tr" class:other={c.other} role="row">
          <span class="who" role="rowheader">{c.label}</span>
          <span role="cell">{fmt(c.f.kills)} / {fmt(c.f.deaths)}</span>
          <b role="cell">{c.f.kd.toFixed(2)}</b>
          <span role="cell">{acc(c.f)}</span>
        </div>
      {/each}
    </div>

    <label class="pick">
      <span>{t('Comparer avec un joueur')}</span>
      <select bind:value={other}>
        <option value="">{t('Choisir un joueur…')}</option>
        {#each others as o (o.key)}<option value={o.key}>{o.name}</option>{/each}
      </select>
    </label>
    {#if cmpMsg}<p class="muted small" aria-live="polite">{cmpMsg}</p>{/if}

    {#if form.recent?.length}
      <h3 class="label sub">{t('10 dernières manches jouées')}</h3>
      <div class="rounds">
        {#each form.recent as r (r.id)}
          <a href={href('match', String(r.id))} title={dt(r.started_at)}>
            <span class="m">{mapName(r.map)}</span>
            <b>{r.kills ?? 0} / {r.deaths ?? 0}</b>
            <small>{r.headshots ?? 0} HS</small>
          </a>
        {/each}
      </div>
    {/if}
  </section>
{/if}

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
  p { margin: 0; }
  .small { font-size: 12px; }
  .trend { font-size: 14px; }
  .trend b { color: var(--danger-text); }
  .trend b.up { color: var(--ok); }
  .tr { display: grid; grid-template-columns: minmax(0, 1.3fr) minmax(0, 1fr) minmax(0, .6fr) minmax(0, .8fr); gap: 8px; align-items: center; padding: 7px 0; border-top: 1px solid var(--line-soft); font-size: 14px; font-variant-numeric: tabular-nums; }
  .tr.head { border-top: 0; padding-top: 0; font-size: 11px; color: var(--muted); font-weight: 700; }
  .tr > :not(:first-child) { text-align: right; }
  .who { font-weight: 700; color: var(--text-2); overflow-wrap: anywhere; line-height: 1.2; }
  .tr.other .who { color: var(--accent); }
  .pick { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .pick select { height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 10px; font-size: 16px; }
  .sub { margin-top: 6px; }
  .rounds { display: grid; grid-template-columns: repeat(auto-fill, minmax(96px, 1fr)); gap: 6px; }
  .rounds a { display: flex; flex-direction: column; padding: 8px 10px; border-radius: 12px; background: var(--surface-2); color: var(--text); text-decoration: none; min-width: 0; }
  .rounds .m { font-size: 12px; color: var(--muted); white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .rounds b { font-variant-numeric: tabular-nums; }
  .rounds small { font-size: 11px; color: var(--muted); }
</style>
