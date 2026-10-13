<script>
  // Commandes programmées of the selected server: a rcon command every N minutes or each day at a time.
  import BackTitle from '../components/BackTitle.svelte';
  import { api, get, post } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { ago } from '../lib/format.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const srv = $derived(currentServer());
  const id = $derived(app.serverId);
  const allowed = $derived(permsOf(app.me, id).has('rcon'));

  let list = $state(null);
  let form = $state({ command: '', mode: 'every', every: 30, at: '04:00' });
  let busy = $state(false);

  const load = sid => get(`/api/servers/${sid}/schedules`).then(r => { list = r.schedules ?? []; }).catch(() => { list = []; });
  $effect(() => { if (id && allowed) load(id); });

  async function add(e) {
    e.preventDefault();
    busy = true;
    try {
      await post(`/api/servers/${id}/schedules`, form.mode === 'every'
        ? { command: form.command.trim(), every_min: Number(form.every) }
        : { command: form.command.trim(), at: form.at });
      form.command = '';
      say(t('Commande programmée'));
      load(id);
    } catch (err) {
      say(t('Échec : {err}', { err: err.message }));
    } finally {
      busy = false;
    }
  }
  async function toggle(s) {
    try { await post(`/api/servers/${id}/schedules/${s.id}/toggle`, {}); load(id); } catch (e) { say(t('Échec : {err}', { err: e.message })); }
  }
  async function del(s) {
    try { await api('DELETE', `/api/servers/${id}/schedules/${s.id}`); load(id); say(t('Supprimée')); } catch (e) { say(t('Échec : {err}', { err: e.message })); }
  }
  const when = s => (s.every_min ? t('toutes les {n} min', { n: s.every_min }) : t('chaque jour à {h}', { h: s.at_time }));
</script>

<BackTitle title={t('Commandes programmées')} sub={srv ? srv.name.replace(/\^\d/g, '') : ''} />

{#if !allowed}
  <p class="muted">{t('Il faut la permission « rcon » sur ce serveur.')}</p>
{:else}
  <form class="card box" onsubmit={add}>
    <label class="fl"><span>{t('Commande rcon')}</span>
      <input bind:value={form.command} required autocomplete="off" autocapitalize="none" spellcheck="false" placeholder={t('ex. : say Rejoignez le Discord !')} />
    </label>
    <div class="modes" role="radiogroup" aria-label={t('Quand')}>
      <button type="button" role="radio" aria-checked={form.mode === 'every'} onclick={() => (form.mode = 'every')}>{t('Régulièrement')}</button>
      <button type="button" role="radio" aria-checked={form.mode === 'at'} onclick={() => (form.mode = 'at')}>{t('Chaque jour')}</button>
    </div>
    {#if form.mode === 'every'}
      <label class="fl inline"><span>{t('Toutes les')}</span><input type="number" min="1" max="10080" bind:value={form.every} /><span>{t('minutes')}</span></label>
    {:else}
      <label class="fl inline"><span>{t('À')}</span><input type="time" bind:value={form.at} required /><span>{t('(heure du monitor)')}</span></label>
    {/if}
    <button class="btn primary" disabled={busy || !form.command.trim()}>{t('Programmer')}</button>
  </form>

  <div class="list">
    {#if list === null}
      <p class="muted">{t('Chargement…')}</p>
    {:else}
      {#each list as s (s.id)}
        <div class="card item" class:off={!s.enabled}>
          <code>{s.command}</code>
          <span class="muted">{when(s)}{#if s.last_run}{' · '}{t('dernière fois il y a {d}', { d: ago(s.last_run) })}{/if}</span>
          <div class="acts">
            <button class="btn" onclick={() => toggle(s)}>{s.enabled ? t('Mettre en pause') : t('Réactiver')}</button>
            <button class="btn ghost" onclick={() => del(s)}>{t('Supprimer')}</button>
          </div>
        </div>
      {:else}
        <p class="muted">{t('Aucune commande programmée.')}</p>
      {/each}
    {/if}
  </div>
{/if}

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; }
  .fl { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); }
  .fl input { height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; color: var(--text); }
  .fl.inline { flex-direction: row; align-items: center; flex-wrap: wrap; }
  .fl.inline input { width: 110px; }
  .modes { display: flex; gap: 6px; }
  .modes button { height: 38px; padding: 0 12px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); font-weight: 700; }
  .modes button[aria-checked='true'] { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .list { display: grid; gap: 10px; margin-top: 12px; }
  .item { padding: 12px 14px; display: flex; flex-direction: column; gap: 6px; }
  .item.off { opacity: .6; }
  .item code { font-size: 14px; overflow-wrap: anywhere; }
  .item .muted { font-size: 13px; }
  .acts { display: flex; gap: 8px; flex-wrap: wrap; }
  .acts .btn { min-height: 38px; font-size: 14px; }
</style>
