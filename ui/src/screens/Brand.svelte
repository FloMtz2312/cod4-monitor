<script>
  // Personnalisation: the color and the logo of the selected server, seen by every visitor (owner only).
  import BackTitle from '../components/BackTitle.svelte';
  import ServerLogo from '../components/ServerLogo.svelte';
  import { api } from '../lib/api.js';
  import { app, currentServer, loadServers } from '../lib/state.svelte.js';
  import { isOwner } from '../lib/perms.js';
  import { SWATCHES, DEFAULT_ACCENT, applyAccent, toSquarePng } from '../lib/brand.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const srv = $derived(currentServer());
  const owner = $derived(isOwner(app.me, app.serverId));
  let accent = $state('');
  let logo = $state(undefined); // undefined: unchanged, '': removed, data URL: new
  let busy = $state(false);

  $effect(() => { accent = srv?.accent ?? ''; logo = undefined; });
  // preview the color right away; leaving without saving puts the saved one back
  $effect(() => {
    applyAccent(accent || DEFAULT_ACCENT);
    return () => applyAccent(currentServer()?.accent);
  });

  const preview = $derived(srv ? { ...srv, accent: accent || null, logo_ts: logo === '' ? null : srv.logo_ts } : null);

  async function pick(e) {
    const f = e.currentTarget.files?.[0];
    if (!f) return;
    try {
      logo = await toSquarePng(f);
    } catch {
      say(t('Image illisible'));
    }
  }

  async function save() {
    busy = true;
    try {
      await api('PUT', `/api/servers/${srv.id}/brand`, logo === undefined ? { accent } : { accent, logo });
      await loadServers();
      logo = undefined;
      say(t('Personnalisation enregistrée'));
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    } finally {
      busy = false;
    }
  }
</script>

<BackTitle title={t('Personnalisation')} sub={srv ? t('Couleur et logo de {name}, vus par tous les visiteurs.', { name: srv.name.replace(/\^\d/g, '') }) : ''} />

{#if !srv || !owner}
  <p class="muted">{t('Réservé au propriétaire du serveur.')}</p>
{:else}
  <section class="card box" aria-label={t('Couleur')}>
    <h2 class="label">{t('Couleur du serveur')}</h2>
    <div class="swatches">
      {#each SWATCHES as [hex, label] (hex)}
        <button class="sw" style:background={hex} aria-label={t(label)} aria-pressed={(accent || DEFAULT_ACCENT).toUpperCase() === hex} onclick={() => (accent = hex === DEFAULT_ACCENT ? '' : hex)}></button>
      {/each}
      <label class="sw custom" aria-label={t('Autre couleur')} style:background={accent && !SWATCHES.some(([h]) => h === accent.toUpperCase()) ? accent : null}>
        <input type="color" value={accent || DEFAULT_ACCENT} oninput={e => (accent = e.currentTarget.value.toUpperCase())} />+
      </label>
    </div>
  </section>

  <section class="card box logo" aria-label={t('Logo')}>
    {#if logo}<img class="new" src={logo} alt="" />{:else}<ServerLogo server={preview} size={56} />{/if}
    <span class="lt"><b>{t('Logo')}</b><span class="muted">{t('Une image carrée de préférence. Sans logo : les initiales.')}</span></span>
    <span class="la">
      <label class="btn">{t('Choisir')}<input class="sr-only" type="file" accept="image/*" onchange={pick} /></label>
      {#if logo || (logo === undefined && srv.logo_ts)}<button class="btn ghost" onclick={() => (logo = '')}>{t('Retirer')}</button>{/if}
    </span>
  </section>

  <section class="card box" aria-label={t('Aperçu')}>
    <h2 class="label">{t('Aperçu')}</h2>
    <div class="pv">
      <ServerLogo server={preview} size={40} />
      <b>{srv.name.replace(/\^\d/g, '')}</b>
      <span class="btn primary small">{t('Rejoindre la partie')}</span>
    </div>
  </section>

  <button class="btn primary save" disabled={busy} onclick={save}>{t('Enregistrer')}</button>
{/if}

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; margin-bottom: 12px; }
  .swatches { display: flex; flex-wrap: wrap; gap: 12px; }
  .sw { width: 44px; height: 44px; border-radius: 14px; border: 3px solid transparent; outline: none; position: relative; }
  .sw[aria-pressed='true'] { border-color: var(--text); }
  .custom { display: flex; align-items: center; justify-content: center; background: var(--surface-2); color: var(--text-2); font-size: 22px; font-weight: 700; cursor: pointer; border: 1px dashed var(--line); }
  .custom input { position: absolute; inset: 0; opacity: 0; width: 100%; height: 100%; cursor: pointer; }
  .logo { flex-direction: row; align-items: center; flex-wrap: wrap; }
  .new { width: 56px; height: 56px; border-radius: 28%; object-fit: cover; }
  .lt { flex: 1; min-width: 160px; display: flex; flex-direction: column; }
  .lt .muted { font-size: 13px; }
  .la { display: flex; gap: 8px; }
  .la .btn { min-height: 40px; cursor: pointer; }
  .pv { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; }
  .pv b { flex: 1; }
  .small { min-height: 36px; font-size: 13px; pointer-events: none; }
  .save { width: 100%; }
</style>
