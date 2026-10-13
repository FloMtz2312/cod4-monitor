<script>
  // Dense sortable table (PC): a name column (snippet), then numeric columns; a click on a header sorts by it.
  import { t } from '../lib/i18n.svelte.js';

  /**
   * @type {{ rows: any[], cols: { k: string, label: string, title?: string, v: (x: any) => any, f?: (x: any) => string, strong?: boolean, text?: boolean }[], name: import('svelte').Snippet<[any]>, nameLabel?: string, sort?: string, rank?: boolean,
   *   keyOf: (x: any) => string, mine?: string, label: string, empty?: string }}
   */
  let { rows, cols, name, nameLabel = t('Joueur'), sort = '', rank = true, keyOf, mine = '', label, empty = '' } = $props();

  let by = $state('');
  let asc = $state(false);
  const key = $derived(by || sort);

  const sorted = $derived.by(() => {
    const c = cols.find(x => x.k === key);
    if (!c) return rows;
    const dir = asc ? 1 : -1;
    return [...rows].sort((a, b) => {
      const va = c.v(a), vb = c.v(b);
      if (va == null || va === '') return vb == null || vb === '' ? 0 : 1;
      if (vb == null || vb === '') return -1;
      return (typeof va === 'string' ? va.localeCompare(vb) : va - vb) * dir;
    });
  });

  function pick(c) {
    if (key === c.k) asc = !asc;
    else { by = c.k; asc = !!c.text; }
  }
</script>

<div class="wrap">
  <table aria-label={label}>
    <thead>
      <tr>
        {#if rank}<th class="n rk">#</th>{/if}
        <th class="nm">{nameLabel}</th>
        {#each cols as c (c.k)}
          <th class:n={!c.text} title={c.title} aria-sort={key === c.k ? (asc ? 'ascending' : 'descending') : undefined}>
            <button onclick={() => pick(c)}>{c.label}{#if key === c.k}<span class="ar">{asc ? '▲' : '▼'}</span>{/if}</button>
          </th>
        {/each}
      </tr>
    </thead>
    <tbody>
      {#each sorted as x, i (keyOf(x))}
        <tr class:mine={mine && keyOf(x) === mine}>
          {#if rank}<td class="n rk">{i + 1}</td>{/if}
          <td class="nm">{@render name(x)}</td>
          {#each cols as c (c.k)}
            <td class:n={!c.text} class:strong={c.strong}>{c.f ? c.f(x) : (c.v(x) ?? '—')}</td>
          {/each}
        </tr>
      {:else}
        <tr><td class="empty" colspan={cols.length + (rank ? 2 : 1)}>{empty || t('Aucune donnée.')}</td></tr>
      {/each}
    </tbody>
  </table>
</div>

<style>
  .wrap { overflow-x: auto; }
  table { width: 100%; border-collapse: collapse; font-size: 13px; }
  th { text-align: left; font-size: 11px; font-weight: 800; color: var(--muted); letter-spacing: .03em; white-space: nowrap; padding: 0 6px 6px; border-bottom: 1px solid var(--line); }
  th button { border: 0; background: transparent; color: inherit; font: inherit; padding: 4px 0; cursor: pointer; letter-spacing: inherit; }
  th[aria-sort] { color: var(--accent); }
  .ar { margin-left: 3px; font-size: 9px; }
  td { padding: 7px 6px; border-top: 1px solid var(--line-soft); white-space: nowrap; font-variant-numeric: tabular-nums; color: var(--text-2); }
  tr:first-child td { border-top: 0; }
  .n { text-align: right; }
  th.n button { text-align: right; }
  .rk { width: 28px; color: var(--muted); }
  td.nm { color: var(--text); font-weight: 700; max-width: 240px; overflow: hidden; text-overflow: ellipsis; }
  .strong { color: var(--text); font-weight: 800; }
  tr.mine td { background: var(--accent-soft); }
  tbody tr:hover td { background: var(--surface-2); }
  .empty { color: var(--muted); padding: 14px 6px; }
</style>
