<script>
  // The player's matches of the last 30 days: result, map, when, their kills / deaths.
  import { mapName } from '../../lib/state.svelte.js';
  import { agoText } from '../../lib/format.js';
  import { href } from '../../lib/router.svelte.js';
  import { t } from '../../lib/i18n.svelte.js';

  /** @type {{ matches: any[] | null }} */
  let { matches } = $props();

  const res = r => (r === 'win' ? 'V' : r === 'loss' ? 'D' : '–');
  const resLabel = r => (r === 'win' ? t('victoire') : r === 'loss' ? t('défaite') : t('sans résultat'));
</script>

{#if matches === null}
  <p class="muted">{t('Chargement…')}</p>
{:else if !matches.length}
  <section class="card box"><p class="muted">{t('Aucun match ces 30 derniers jours.')}</p></section>
{:else}
  <div class="card list">
    {#each matches as m (m.match.id)}
      <a class="row" href={href('match', String(m.match.id))}>
        <span class="res {m.player?.result}" title={resLabel(m.player?.result)}>{res(m.player?.result)}<span class="sr-only"> {resLabel(m.player?.result)}</span></span>
        <span class="mid">
          <b>{mapName(m.match.map)}</b>
          <span class="muted">{agoText(m.match.ended_at || m.match.started_at)}{#if m.score_allies != null}{' · '}{m.score_allies}–{m.score_axis}{/if}</span>
        </span>
        <span class="kd"><b>{m.player?.kills ?? 0} / {m.player?.deaths ?? 0}</b><small>{t('kills / morts')}</small></span>
      </a>
    {/each}
  </div>
{/if}

<style>
  .box { padding: 16px; }
  .box p { margin: 0; }
  .list { overflow: hidden; }
  .row { display: flex; align-items: center; gap: 12px; min-height: 62px; padding: 8px 14px; border-top: 1px solid var(--line-soft); text-decoration: none; color: var(--text); }
  .row:first-child { border-top: 0; }
  .res { width: 34px; height: 34px; flex: none; border-radius: 10px; display: flex; align-items: center; justify-content: center; font-weight: 800; background: var(--surface-2); color: var(--text-2); }
  .res.win { background: var(--accent-soft); color: var(--accent); }
  .mid { flex: 1; min-width: 0; display: flex; flex-direction: column; }
  .mid .muted { font-size: 13px; }
  .kd { display: flex; flex-direction: column; align-items: flex-end; }
  .kd b { font-variant-numeric: tabular-nums; }
  .kd small { font-size: 11px; color: var(--muted); }
</style>
