<script>
  // Bans du serveur (permission "ban"): the active bans as the game server lists them (dumpbanlist), unban by id,
  // and the bans issued from the site.
  import BackTitle from '../components/BackTitle.svelte';
  import CodName from '../components/CodName.svelte';
  import { get, post } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { href } from '../lib/router.svelte.js';
  import { clean } from '../lib/format.js';
  import { dt } from '../lib/moderation.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const srv = $derived(currentServer());
  const allowed = $derived(permsOf(app.me, id).has('ban'));
  let data = $state(null);
  let err = $state('');
  let unbanId = $state('');
  let sure = $state(false);
  let busy = $state(false);

  const load = sid => get(`/api/servers/${sid}/bans`).then(r => { data = r; err = ''; }).catch(e => { data = { active: '', issued: [] }; err = t(e.message); });
  $effect(() => {
    const sid = id;
    if (!sid || !allowed) return;
    data = null;
    load(sid);
  });

  // CoD4X keeps bans only with a banlist plugin (simplebanlist), which also brings dumpbanlist
  const noPlugin = $derived(/bad command|unknown command/i.test(data?.active ?? ''));
  const active = $derived(clean(data?.active ?? '').trim());

  async function unban(e) {
    e.preventDefault();
    const v = unbanId.trim();
    if (!v) return;
    if (!sure) { sure = true; return; }
    busy = true;
    try {
      const r = await post(`/api/servers/${id}/unban`, { id: v });
      say(clean(r.output || '').trim() || t('{id} débanni', { id: v }));
      unbanId = '';
      load(id);
    } catch (e2) {
      say(t('Échec : {err}', { err: t(e2.message) }));
    } finally {
      busy = false;
      sure = false;
    }
  }
</script>

<BackTitle title={t('Bans du serveur')} sub={srv ? clean(srv.name) : ''} fallback="moderation" />

{#if !allowed}
  <p class="muted">{t('Il faut la permission « ban » sur ce serveur.')}</p>
{:else if data === null}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <div class="grid">
    <section class="card box">
      <h2 class="label">{t('Bans actifs sur le serveur')}</h2>
      {#if err}
        <p class="warn">{err}</p>
      {:else if noPlugin}
        <p class="warn">{t('Liste des bans indisponible : le plugin simplebanlist n\'est pas chargé sur le serveur. Sans lui, CoD4X ne garde aucun ban : un ban éjecte le joueur, qui peut revenir. Ajoute « loadplugin simplebanlist » dans server.cfg (fichier simplebanlist dans le dossier plugins du serveur).')}</p>
      {:else}
        <pre class="out">{active || t('Aucun ban actif')}</pre>
      {/if}
      <form class="line" onsubmit={unban}>
        <label class="sr-only" for="unban-id">{t('Identifiant à débannir')}</label>
        <input id="unban-id" bind:value={unbanId} oninput={() => (sure = false)} placeholder={t('GUID ou id à débannir')} autocomplete="off" autocapitalize="none" spellcheck="false" />
        <button class="btn" class:danger={sure} disabled={busy || !unbanId.trim()}>{sure ? t('Confirmer') : t('Débannir')}</button>
      </form>
    </section>

    <section class="card box">
      <h2 class="label">{t('Bans donnés depuis le site')}</h2>
      {#each data.issued ?? [] as b, i (i)}
        <div class="ban">
          <span class="who">
            {#if b.guid}<a href={href('joueur', b.guid)}><CodName raw={b.name_raw} /></a>{:else}<CodName raw={b.name_raw} />{/if}
            <span class="dur">{b.duration || t('définitif')}</span>
          </span>
          <span class="muted small">{dt(b.ts)}{#if b.reason}{' · '}{b.reason}{/if}</span>
        </div>
      {:else}
        <p class="muted">{t('Aucun ban depuis le site.')}</p>
      {/each}
    </section>
  </div>
{/if}

<style>
  .grid { display: grid; gap: 12px; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; min-width: 0; }
  .box p { margin: 0; }
  .warn { color: var(--warn); font-size: 14px; }
  .out { margin: 0; border-radius: 12px; background: #0A0D12; border: 1px solid var(--line); padding: 10px 12px; max-height: 260px; overflow: auto; font-size: 12px; line-height: 1.45; white-space: pre-wrap; word-break: break-word; }
  .line { display: flex; gap: 8px; }
  .line input { flex: 1; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; font-family: ui-monospace, monospace; }
  .line .btn { min-height: 44px; }
  .ban { display: flex; flex-direction: column; gap: 2px; padding-top: 8px; border-top: 1px solid var(--line-soft); }
  .label + .ban { border-top: 0; padding-top: 0; }
  .who { display: flex; align-items: center; gap: 8px; min-width: 0; font-weight: 700; }
  .who a { color: var(--text); text-decoration: none; display: inline-flex; min-width: 0; }
  .dur { font-size: 11px; font-weight: 800; padding: 2px 7px; border-radius: 7px; background: rgba(255, 99, 105, .14); color: var(--danger-text); flex: none; }
  .small { font-size: 12px; overflow-wrap: anywhere; }
  @media (min-width: 960px) { .grid { grid-template-columns: 1fr 1fr; align-items: start; } }
</style>
