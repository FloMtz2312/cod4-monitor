<script>
  // Duel: who killed whom between two players (all time). #/duel/<a>/<b>, "_" for a side not chosen yet.
  import BackTitle from '../components/BackTitle.svelte';
  import CodName from '../components/CodName.svelte';
  import Icon from '../components/Icon.svelte';
  import { get } from '../lib/api.js';
  import { app } from '../lib/state.svelte.js';
  import { route, replace, href } from '../lib/router.svelte.js';
  import { clean, dateTime, weaponName, fmt } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const side = i => { const v = route.params[i] ?? ''; return v === '_' ? '' : v; };
  const a = $derived(side(0));
  const b = $derived(side(1));

  let players = $state(/** @type {any[] | null} */ (null));
  let d = $state(/** @type {any} */ (null));
  let failed = $state('');
  let qa = $state('');
  let qb = $state('');

  $effect(() => {
    const sid = id;
    if (!sid) return;
    let alive = true;
    players = null;
    get(`/api/servers/${sid}/players?range=90d`).then(r => { if (alive) players = r.players ?? []; }).catch(() => { if (alive) players = []; });
    return () => { alive = false; };
  });

  $effect(() => {
    const sid = id, ka = a, kb = b;
    d = null;
    failed = '';
    if (!sid || !ka || !kb || ka === kb) return;
    let alive = true;
    get(`/api/servers/${sid}/duel?a=${encodeURIComponent(ka)}&b=${encodeURIComponent(kb)}`)
      .then(r => { if (alive) d = r; })
      .catch(e => { if (alive) failed = e.message; });
    return () => { alive = false; };
  });

  const byKey = $derived(Object.fromEntries((players ?? []).map(p => [p.key, p])));
  const nameOf = (k, raw) => raw || byKey[k]?.name_raw || (k.startsWith('n:') ? k.slice(2) : k);
  const A = $derived(d?.a?.[0] ?? null);
  const B = $derived(d?.b?.[0] ?? null);
  const total = $derived((A?.kills ?? 0) + (B?.kills ?? 0));

  function matches(q, other) {
    const s = clean(q).trim().toLowerCase();
    if (!s) return [];
    return (players ?? []).filter(p => p.key !== other && ((p.name_clean ?? '').toLowerCase().includes(s) || (p.aliases ?? '').toLowerCase().includes(s))).slice(0, 8);
  }
  const listA = $derived(matches(qa, b));
  const listB = $derived(matches(qb, a));

  function choose(which, key) {
    const na = which === 'a' ? key : a, nb = which === 'b' ? key : b;
    if (which === 'a') qa = ''; else qb = '';
    replace('duel', na || '_', nb || '_');
  }
  const swap = () => replace('duel', b || '_', a || '_');
</script>

<BackTitle title={t('Duel')} sub={t('Qui a tué qui entre deux joueurs, depuis le début.')} fallback="classements" />

{#snippet picker(which, key, list)}
  <div class="pick card">
    <span class="label">{which === 'a' ? t('Joueur 1') : t('Joueur 2')}</span>
    {#if key}
      <div class="chosen">
        <a class="pn" href={href('joueur', key)}><CodName raw={nameOf(key, '')} /></a>
        <button class="btn ghost x" onclick={() => choose(which, '')} aria-label={t('Changer de joueur')}>{t('Changer')}</button>
      </div>
    {:else}
      <label class="search">
        <Icon name="search" size={18} />
        <span class="sr-only">{t('Rechercher un joueur')}</span>
        {#if which === 'a'}
          <input type="search" bind:value={qa} placeholder={t('Pseudo, ancien pseudo…')} autocomplete="off" spellcheck="false" />
        {:else}
          <input type="search" bind:value={qb} placeholder={t('Pseudo, ancien pseudo…')} autocomplete="off" spellcheck="false" />
        {/if}
      </label>
      {#if players === null}
        <p class="muted small">{t('Chargement…')}</p>
      {/if}
      {#each list as p (p.key)}
        <button class="opt" onclick={() => choose(which, p.key)}><CodName raw={p.name_raw} /></button>
      {/each}
    {/if}
  </div>
{/snippet}

<div class="pickers">
  {@render picker('a', a, listA)}
  <button class="swap" onclick={swap} aria-label={t('Inverser')} title={t('Inverser')} disabled={!a && !b}>⇄</button>
  {@render picker('b', b, listB)}
</div>

{#if failed}
  <section class="card res"><p class="muted">{t(failed)}</p></section>
{:else if a && b && !d}
  <p class="muted">{t('Chargement…')}</p>
{:else if d}
  <section class="card res" aria-label={t('Résultat du duel')}>
    {#if total}
      <div class="score">
        <span class="nm l"><CodName raw={nameOf(a, A?.name_raw)} /></span>
        <b><span class="ca">{A.kills}</span> – <span class="cb">{B.kills}</span></b>
        <span class="nm r"><CodName raw={nameOf(b, B?.name_raw)} /></span>
      </div>
      <div class="bar"><div class="ba" style:flex={A.kills || 0.0001}></div><div class="bb" style:flex={B.kills || 0.0001}></div></div>
      <table>
        <tbody>
          <tr><td class="n">{fmt(A.headshots ?? 0)}</td><th>{t('Headshots')}</th><td>{fmt(B.headshots ?? 0)}</td></tr>
          <tr><td class="n">{A.weapon ? weaponName(A.weapon) : '—'}</td><th>{t('Arme')}</th><td>{B.weapon ? weaponName(B.weapon) : '—'}</td></tr>
          <tr><td class="n">{A.last ? dateTime(A.last) : '—'}</td><th>{t('Dernier kill')}</th><td>{B.last ? dateTime(B.last) : '—'}</td></tr>
        </tbody>
      </table>
    {:else}
      <p class="muted">{t('Ces deux joueurs ne se sont jamais tués.')}</p>
    {/if}
  </section>
{:else}
  <p class="muted hint">{t('Choisis deux joueurs pour comparer leurs face-à-face.')}</p>
{/if}

<style>
  .pickers { display: grid; gap: 8px; }
  .pick { padding: 12px 14px; display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .chosen { display: flex; align-items: center; gap: 8px; min-height: 44px; }
  .pn { flex: 1; min-width: 0; display: flex; overflow: hidden; white-space: nowrap; font-family: var(--font-display); font-size: 20px; font-weight: 700; color: var(--text); text-decoration: none; }
  .x { min-height: 36px; color: var(--accent); }
  .search { display: flex; align-items: center; gap: 8px; height: 46px; padding: 0 12px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); color: var(--muted); }
  .search:focus-within { border-color: var(--accent); }
  .search input { flex: 1; min-width: 0; height: 42px; border: 0; background: transparent; color: var(--text); font-size: 16px; outline: none; }
  .opt { display: flex; align-items: center; min-height: 42px; padding: 0 6px; border: 0; border-top: 1px solid var(--line-soft); background: transparent; color: var(--text); font-weight: 700; text-align: left; }
  .opt:hover { background: var(--surface-2); }
  .small { font-size: 12px; margin: 0; }
  .swap { justify-self: center; width: 44px; height: 36px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); color: var(--text-2); font-size: 18px; }
  .res { margin-top: 12px; padding: 16px; }
  .res p { margin: 0; }
  .score { display: grid; grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); align-items: center; gap: 10px; }
  .nm { display: flex; min-width: 0; overflow: hidden; white-space: nowrap; font-weight: 700; }
  .nm.r { justify-content: flex-end; }
  .score b { font-family: var(--font-display); font-size: 34px; font-variant-numeric: tabular-nums; }
  .ca { color: var(--accent); }
  .cb { color: var(--axis); }
  .bar { display: flex; height: 10px; border-radius: 5px; overflow: hidden; margin: 12px 0; gap: 2px; }
  .ba { background: var(--accent); }
  .bb { background: var(--axis); }
  table { width: 100%; border-collapse: collapse; font-size: 14px; }
  th { font-size: 12px; color: var(--muted); font-weight: 700; text-align: center; padding: 8px 4px; }
  td { padding: 8px 4px; border-top: 1px solid var(--line-soft); font-variant-numeric: tabular-nums; }
  th { border-top: 1px solid var(--line-soft); }
  tr:first-child td, tr:first-child th { border-top: 0; }
  td.n { text-align: left; }
  td:last-child { text-align: right; }
  .hint { margin-top: 14px; }
  @media (min-width: 960px) {
    .pickers { grid-template-columns: minmax(0, 1fr) auto minmax(0, 1fr); align-items: start; }
    .swap { margin-top: 30px; }
    .res { max-width: 720px; }
  }
</style>
