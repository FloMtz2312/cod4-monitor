<script>
  // Connected players, by team when the logs tell it (else one list), best score first.
  import CodName from './CodName.svelte';
  import { pingColor } from '../lib/format.js';
  import { dur } from '../lib/dates.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ groups: {id: string, name: string, color: string, players: any[]}[], ontap: (p: any) => void, hint: string, meKey?: string, tagOf?: (p: any) => string, now?: number }} */
  let { groups, ontap, hint, meKey = '', tagOf = () => '', now = Date.now() / 1000 } = $props();
  // staff tags of the notes (private): short marker next to the name
  const TAG = { suspect: '⚠ Suspect', surveiller: '👁 À surveiller', vip: '★ VIP', ami: '♥ Ami' };
</script>

<div class="groups">
  {#each groups as g (g.id)}
    <section class="card team" aria-label={g.name}>
      <div class="th">
        <span class="sw" style:background={g.color}></span>
        <span class="tn">{g.name}</span>
        <span class="tc">{g.players.length}</span>
        <span class="cols" aria-hidden="true"><span>K/M</span><span>Score</span><span>Ping</span></span>
      </div>
      {#each g.players as p, i (p.name_raw)}
        <button class="row" class:me={!!meKey && p.key === meKey} onclick={() => ontap(p)} aria-label={t('{name} : {k} kills, {d} morts, ping {p} ms', { name: p.name_clean, k: p.kills ?? 0, d: p.deaths ?? 0, p: p.ping ?? '—' })}>
          <span class="rk">{i + 1}</span>
          <span class="nm">
            <span class="l1"><CodName raw={p.name_raw} />{#if meKey && p.key === meKey}<span class="you">{t('toi')}</span>{/if}{#if tagOf(p)}<span class="tag {tagOf(p)}">{t(TAG[tagOf(p)] ?? tagOf(p))}</span>{/if}</span>
            {#if p.started_at}<span class="since">{t('depuis {d}', { d: dur(now - p.started_at) })}</span>{/if}
          </span>
          <span class="kd">{p.kills ?? '–'}/{p.deaths ?? '–'}</span>
          <span class="sc">{p.score ?? 0}</span>
          <span class="pg"><span class="dot" style:background={pingColor(p.ping)}></span>{p.ping ?? '—'}</span>
        </button>
      {/each}
    </section>
  {/each}
  {#if hint}<p class="hint">{hint}</p>{/if}
</div>

<style>
  .groups { display: flex; flex-direction: column; gap: 12px; }
  .team { overflow: hidden; }
  .th { display: flex; align-items: center; gap: 8px; padding: 12px 14px 8px; }
  .sw { width: 10px; height: 10px; border-radius: 3px; }
  .tn { font-weight: 800; font-size: 14px; }
  .tc { color: var(--muted); font-size: 13px; }
  .cols { margin-left: auto; display: flex; gap: 0; font-size: 11px; color: var(--muted); font-weight: 700; }
  .cols span { text-align: right; }
  .cols span:nth-child(1) { width: 52px; } .cols span:nth-child(2) { width: 48px; } .cols span:nth-child(3) { width: 54px; }
  .row {
    width: 100%; display: flex; align-items: center; height: 50px; padding: 0 14px; border: 0; border-top: 1px solid var(--line-soft);
    background: transparent; text-align: left; font-size: 14px;
  }
  .rk { width: 22px; flex: none; color: var(--muted); font-size: 12px; font-weight: 700; }
  .nm { flex: 1; min-width: 0; display: flex; flex-direction: column; justify-content: center; font-weight: 700; }
  .l1 { display: flex; align-items: center; gap: 6px; min-width: 0; }
  .since { font-size: 11px; font-weight: 600; color: var(--muted); }
  .tag { flex: none; font-size: 10px; font-weight: 800; border-radius: 6px; padding: 1px 5px; background: var(--surface-2); color: var(--text-2); }
  .tag.suspect { background: rgba(255, 99, 105, .14); color: var(--danger-text); }
  .tag.surveiller { background: rgba(245, 196, 81, .15); color: var(--warn); }
  .row.me { background: var(--accent-soft); }
  .you { flex: none; font-size: 11px; font-weight: 800; color: var(--on-accent); background: var(--accent); border-radius: 6px; padding: 1px 6px; }
  .kd { width: 52px; text-align: right; color: var(--text-2); font-variant-numeric: tabular-nums; }
  .sc { width: 48px; text-align: right; font-weight: 800; font-variant-numeric: tabular-nums; }
  .pg { width: 54px; display: flex; align-items: center; justify-content: flex-end; gap: 5px; color: var(--text-2); font-size: 13px; font-variant-numeric: tabular-nums; }
  .dot { width: 7px; height: 7px; border-radius: 4px; }
  .hint { margin: 0; font-size: 12px; color: var(--muted); text-align: center; }
</style>
