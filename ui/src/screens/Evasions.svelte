<script>
  // Contournements de ban (private data): every player active in the last 30 days that looks like a banned player,
  // with the clues and their points (alert from the threshold); and B3's own list (same IP as a banned player).
  import BackTitle from '../components/BackTitle.svelte';
  import CodName from '../components/CodName.svelte';
  import B3PenaltySheet from '../components/B3PenaltySheet.svelte';
  import { get } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf, isOwner } from '../lib/perms.js';
  import { href } from '../lib/router.svelte.js';
  import { dt, CLUES, PEN_LABEL } from '../lib/moderation.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const srv = $derived(currentServer());
  const allowed = $derived(permsOf(app.me, id).has('private'));
  const owner = $derived(isOwner(app.me, id));
  let list = $state(null);
  let alert = $state(50);
  let b3 = $state(null); // null: no B3
  let target = $state(null);

  function load(sid) {
    get(`/api/servers/${sid}/evasion`).then(r => { list = r.evasions ?? []; alert = r.alert ?? 50; }).catch(() => { list = []; });
    get(`/api/servers/${sid}/b3/evasions`).then(r => { b3 = r.evasions ?? []; }).catch(() => { b3 = null; });
  }
  $effect(() => {
    const sid = id;
    if (!sid || !allowed) return;
    list = null;
    b3 = null;
    load(sid);
  });
</script>

<BackTitle title={t('Contournements de ban')} sub={t('Joueurs actifs ces 30 derniers jours comparés aux joueurs bannis · alerte à partir de {n}', { n: alert })} fallback="moderation" />

{#if !allowed}
  <p class="muted">{t('Il faut l\'accès aux données privées de ce serveur.')}</p>
{:else if list === null}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <div class="list">
    {#each list as x (x.key + x.ban_guid)}
      <article class="card item">
        <div class="top">
          <h2>
            <a href={href('joueur', x.key)}><CodName raw={x.name_raw} /></a>
            <span class="muted">{t('ressemble à')}</span>
            {#if x.ban_guid}<a href={href('joueur', x.ban_guid)}><CodName raw={x.ban_name} /></a>{:else}<CodName raw={x.ban_name} />{/if}
          </h2>
          <span class="score" class:hot={x.score >= alert} title={t('Score')}>{x.score}</span>
        </div>
        <div class="clues">
          {#each x.reasons ?? [] as r (r.code)}
            <span class="clue"><b>+{r.points}</b> {t(CLUES[r.code] ?? r.code)}{#if r.detail}<span class="muted">{' · '}{r.detail}</span>{/if}</span>
          {/each}
        </div>
        <p class="muted small">{t('Banni le {d}', { d: dt(x.ban_ts) })} ({x.duration || t('définitif')}) · {t('vu le {d}', { d: dt(x.last_seen) })}</p>
      </article>
    {:else}
      <section class="card empty"><p class="muted">{t('Rien de suspect')}</p></section>
    {/each}
  </div>

  {#if b3}
    <h2 class="label sec">{t('Contournements B3')}</h2>
    <p class="muted small intro">{t('Joueurs non bannis venus depuis l\'IP d\'un banni après son ban.')}</p>
    <div class="list">
      {#each b3 as e, i (i)}
        <article class="card item">
          <h2>
            <a href={href('joueur', e.suspect.guid)}><CodName raw={e.suspect.name} /></a>
            <span class="muted">{t('même IP que')}</span>
            <a href={href('joueur', e.banned.guid)}><CodName raw={e.banned.name} /></a>
          </h2>
          <p class="small">
            <span class="kind">{t(PEN_LABEL[e.ban_type] ?? e.ban_type)}</span> {dt(e.ban_at)}{#if e.reason}{' · '}{e.reason}{/if}
          </p>
          <p class="muted small">{t('IP communes : {ips}', { ips: (e.shared ?? []).join(', ') })} · {t('vu le {d}', { d: dt(e.suspect.time_edit) })}</p>
          {#if owner}<div><button class="btn danger" onclick={() => (target = e.suspect)}>{t('Bannir')}</button></div>{/if}
        </article>
      {:else}
        <section class="card empty"><p class="muted">{t('Aucun joueur suspect')}</p></section>
      {/each}
    </div>
  {/if}
{/if}

{#if target}
  <B3PenaltySheet open={!!target} onclose={() => (target = null)} serverId={id} guid={target.guid} nameRaw={target.name} ondone={() => load(id)} />
{/if}

<style>
  .list { display: grid; gap: 10px; }
  .item { padding: 14px 16px; display: flex; flex-direction: column; gap: 8px; min-width: 0; }
  .item p { margin: 0; }
  .top { display: flex; justify-content: space-between; align-items: flex-start; gap: 10px; }
  h2 { font-size: 15px; font-weight: 800; display: flex; flex-wrap: wrap; gap: 5px; align-items: baseline; min-width: 0; }
  h2 a { color: var(--text); text-decoration: none; display: inline-flex; min-width: 0; max-width: 100%; }
  h2 .muted { font-weight: 600; }
  .score { flex: none; min-width: 40px; text-align: center; font-family: var(--font-display); font-weight: 700; font-size: 18px; padding: 2px 8px; border-radius: 10px; background: var(--accent-soft); color: var(--accent); }
  .score.hot { background: rgba(255, 99, 105, .15); color: var(--danger-text); }
  .clues { display: flex; flex-wrap: wrap; gap: 6px; }
  .clue { font-size: 12px; padding: 3px 8px; border-radius: 8px; background: var(--surface-2); color: var(--text-2); overflow-wrap: anywhere; }
  .clue b { color: var(--text); }
  .small { font-size: 12px; overflow-wrap: anywhere; }
  .kind { font-size: 11px; font-weight: 800; padding: 2px 7px; border-radius: 7px; background: rgba(255, 99, 105, .14); color: var(--danger-text); }
  .empty { padding: 14px 16px; }
  .empty p { margin: 0; }
  .sec { display: block; margin: 20px 2px 4px; }
  .intro { margin: 0 2px 10px; }
  .btn { min-height: 40px; font-size: 14px; }
  @media (min-width: 960px) { .list { grid-template-columns: repeat(2, minmax(0, 1fr)); align-items: start; } }
</style>
