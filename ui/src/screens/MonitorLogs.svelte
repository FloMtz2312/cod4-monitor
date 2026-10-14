<script>
  // Logs du monitor (super admin): the last lines the monitor logged, the same as "docker logs" (passwords and keys
  // masked by the server). Newest first, errors only or everything, refreshed every 5 s while the page is open.
  import BackTitle from '../components/BackTitle.svelte';
  import Seg from '../components/Seg.svelte';
  import Icon from '../components/Icon.svelte';
  import { get } from '../lib/api.js';
  import { say } from '../lib/toast.svelte.js';
  import { dateTime } from '../lib/format.js';
  import { t } from '../lib/i18n.svelte.js';

  let lines = $state(null);
  let failed = $state('');
  let show = $state('all'); // all | problems
  let query = $state('');
  let live = $state(true);
  let shown = $state(200);

  async function load() {
    try {
      lines = (await get('/api/monitor-logs')).lines ?? [];
      failed = '';
    } catch (e) {
      failed = e.message;
      lines ??= [];
    }
  }
  $effect(() => {
    load();
    if (!live) return;
    const timer = setInterval(load, 5000);
    return () => clearInterval(timer);
  });

  // a JSON line of the monitor: time, level, msg and the other fields
  function parse(raw) {
    try {
      const o = JSON.parse(raw);
      if (o && typeof o === 'object') {
        const { time, level, msg, ...rest } = o;
        const extra = Object.entries(rest).map(([k, v]) => `${k}=${typeof v === 'string' ? v : JSON.stringify(v)}`).join(' ');
        return { ts: time ? Date.parse(time) / 1000 : 0, level: String(level ?? 'INFO'), msg: String(msg ?? ''), extra, raw };
      }
    } catch {}
    return { ts: 0, level: /error|fatal|panic/i.test(raw) ? 'ERROR' : 'INFO', msg: raw, extra: '', raw };
  }

  const rows = $derived.by(() => {
    const q = query.trim().toLowerCase();
    return (lines ?? []).map(parse).reverse().filter(l =>
      (show === 'all' || l.level === 'ERROR' || l.level === 'WARN') && (!q || l.raw.toLowerCase().includes(q)));
  });

  async function copy() {
    try {
      await navigator.clipboard.writeText(rows.map(l => l.raw).join('\n'));
      say(t('Logs copiés'));
    } catch {
      say(t('Copie impossible'));
    }
  }
</script>

<BackTitle title={t('Logs du monitor')} sub={t('Les dernières lignes du monitor, comme docker logs. Mots de passe et clés masqués, rien n\'est gardé après un redémarrage.')} fallback="plus" />

<div class="tools">
  <Seg label={t('Filtrer les logs')} value={show} onchange={v => (show = v)} items={[['all', t('Tout')], ['problems', t('Erreurs et alertes')]]} />
  <input class="search" type="search" bind:value={query} placeholder={t('Rechercher dans les logs…')} aria-label={t('Rechercher dans les logs')} autocomplete="off" spellcheck="false" />
  <label class="live"><input type="checkbox" bind:checked={live} />{t('Actualiser toutes les 5 s')}</label>
  <button class="btn sm" onclick={copy} disabled={!rows.length}><Icon name="list" size={16} />{t('Copier')}</button>
</div>

{#if failed}<p class="bad">{failed}</p>{/if}
{#if lines === null}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <div class="card list">
    {#each rows.slice(0, shown) as l, i (i)}
      <div class="line" class:err={l.level === 'ERROR'} class:warn={l.level === 'WARN'}>
        <span class="meta">{#if l.ts}{dateTime(l.ts)}{/if} <b>{l.level}</b></span>
        <span class="msg">{l.msg}{#if l.extra}<span class="extra"> {l.extra}</span>{/if}</span>
      </div>
    {:else}
      <p class="muted empty">{t('Aucune ligne.')}</p>
    {/each}
  </div>
  {#if rows.length > shown}<button class="btn more" onclick={() => (shown += 200)}>{t('Afficher plus ({n} restants)', { n: rows.length - shown })}</button>{/if}
{/if}

<style>
  .tools { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; margin-bottom: 12px; }
  .search { flex: 1 1 220px; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text); padding: 0 12px; font-size: 16px; }
  .live { display: flex; align-items: center; gap: 6px; font-size: 13px; color: var(--text-2); }
  .live input { width: 18px; height: 18px; accent-color: var(--accent); }
  .sm { height: 40px; padding: 0 12px; font-size: 13px; gap: 6px; }
  .bad { color: var(--danger-text); }
  .list { overflow: hidden; }
  .line { display: flex; flex-direction: column; gap: 2px; padding: 8px 12px; border-top: 1px solid var(--line-soft); font-family: ui-monospace, monospace; font-size: 12px; }
  .line:first-child { border-top: 0; }
  .meta { color: var(--muted); }
  .line.err .meta b, .line.err .msg { color: var(--danger-text); }
  .line.warn .meta b { color: var(--warn); }
  .msg { overflow-wrap: anywhere; color: var(--text); }
  .extra { color: var(--text-2); }
  .empty { margin: 0; padding: 14px; }
  .more { width: 100%; margin-top: 12px; }
</style>
