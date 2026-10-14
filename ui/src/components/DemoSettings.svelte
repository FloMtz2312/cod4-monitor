<script>
  // Démos of a server (in its settings): record every real player while at least N are on (CoD4X "record", one
  // demo per player, seen from their eyes), keep time, and an estimate of the space it takes to guide the choice.
  import Icon from './Icon.svelte';
  import { api, get } from '../lib/api.js';
  import { say } from '../lib/toast.svelte.js';
  import { gb, size } from '../lib/demos.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ serverId: number }} */
  let { serverId } = $props();

  let data = $state(null);
  let f = $state({ enabled: false, min_players: 2, keep: 48, unit: 'h', dir: '', reserve_gb: 0 });
  let est = $state(null);
  let busy = $state(false);
  let failed = $state('');

  const hours = $derived(Math.max(1, Math.round(Number(f.keep) || 1) * (f.unit === 'd' ? 24 : 1)));

  function fill(r) {
    data = r;
    est = r.estimate;
    const k = r.settings.keep_hours;
    f = { enabled: r.settings.enabled, min_players: r.settings.min_players, keep: k % 24 === 0 && k >= 48 ? k / 24 : k,
      unit: k % 24 === 0 && k >= 48 ? 'd' : 'h', dir: r.settings.dir ?? '', reserve_gb: r.settings.reserve_gb ?? 0 };
  }

  $effect(() => {
    const id = serverId;
    get(`/api/serverconf/${id}/demos`).then(fill).catch(e => { failed = e.message; });
  });

  // live estimate while the threshold, the keep time or the reserve change
  let timer;
  $effect(() => {
    const q = `min=${Number(f.min_players) || 1}&keep=${hours}&reserve=${Number(f.reserve_gb) || 0}`;
    if (!data) return;
    clearTimeout(timer);
    timer = setTimeout(() => get(`/api/serverconf/${serverId}/demos/estimate?${q}`).then(e => { est = e; }).catch(() => {}), 300);
    return () => clearTimeout(timer);
  });

  async function save(e) {
    e.preventDefault();
    busy = true;
    try {
      fill(await api('PUT', `/api/serverconf/${serverId}/demos`, { enabled: f.enabled, min_players: Number(f.min_players), keep_hours: hours,
        dir: f.dir.trim(), reserve_gb: Number(f.reserve_gb) || 0 }));
      say(t('Réglages des démos enregistrés'));
    } catch (err) {
      say(t('Échec : {err}', { err: err.message }));
    } finally {
      busy = false;
    }
  }

  const st = $derived(data?.status ?? {});
  const days = n => (n >= 2 ? t('{n} jours', { n: Math.floor(n) }) : t('{n} h', { n: Math.floor(n * 24) }));
  const num = (n, d = 1) => (n ?? 0).toLocaleString(document.documentElement.lang === 'en' ? 'en-US' : 'fr-FR', { maximumFractionDigits: d });
</script>

<section class="card box">
  <h2 class="label">{t('Démos')}</h2>
  {#if failed}
    <p class="bad">{failed}</p>
  {:else if !data}
    <p class="muted">{t('Chargement…')}</p>
  {:else}
    <p class="muted small">{t('Le serveur enregistre une démo par joueur, vue par ses yeux, dès qu\'il y a assez de joueurs. Une démo liée à un signalement, un ban ou une alerte est gardée 90 jours et s\'ouvre en un clic depuis la Modération.')}</p>
    <form onsubmit={save}>
      <label class="switch">
        <input type="checkbox" bind:checked={f.enabled} disabled={!data.has_rcon} />
        <span><b>{t('Enregistrer automatiquement les démos de tous les joueurs')}</b>
          <small>{data.has_rcon ? t('les bots ne sont pas enregistrés') : t('Il faut le mot de passe rcon du serveur.')}</small></span>
      </label>

      {#if data.settings.enabled || data.usage.count}
        <p class="state" class:on={st.active}>
          <Icon name={st.active ? 'live' : 'clock'} size={16} />
          {#if !data.settings.enabled}{t('Désactivé')}
          {:else if st.error === 'rcon'}{t('En pause : pas de mot de passe rcon')}
          {:else if st.error}{t('Erreur rcon : {err}', { err: st.error })}
          {:else if st.active}{t('Enregistrement en cours : {n} joueurs', { n: st.players })}
          {:else}{t('En attente : {n} / {min} joueurs', { n: st.players ?? 0, min: data.settings.min_players })}{/if}
          {#if data.usage.count}{' · '}{t('{n} démos, {size}', { n: data.usage.count, size: size(data.usage.bytes) })}{#if data.usage.kept}{' · '}{t('{n} gardées', { n: data.usage.kept })}{/if}{/if}
        </p>
        {#if st.files_error}<p class="bad small">{t('Dossier des démos : {err}', { err: st.files_error })}</p>{/if}
      {/if}

      <div class="grid">
        <label class="fl"><span>{t('À partir de (joueurs)')}</span><input type="number" min="1" max="64" bind:value={f.min_players} /></label>
        <div class="fl"><span>{t('Garder les démos non liées pendant')}</span>
          <div class="keep">
            <input type="number" min="1" max={f.unit === 'd' ? 365 : 8760} bind:value={f.keep} aria-label={t('Durée')} />
            <select bind:value={f.unit} aria-label={t('Unité')}><option value="h">{t('heures')}</option><option value="d">{t('jours')}</option></select>
          </div>
        </div>
      </div>

      {#if est}
        <div class="est" class:over={est.over}>
          <p>{#if est.history_days >= 1}{t('Ton serveur : {p} joueurs en moyenne, {h} h par jour à {min} joueurs ou plus (sur {d} jours).', { p: num(est.avg_players), h: num(est.hours_per_day), min: Number(f.min_players) || 1, d: Math.min(30, Math.floor(est.history_days)) })}{:else}{t('Pas encore assez d\'historique sur ce serveur pour estimer l\'affluence.')}{/if}
            {est.measured ? t('Une démo pèse ~{mb} Mo par joueur et par heure (mesuré sur tes démos).', { mb: num(est.mb_per_hour, 0) }) : t('Une démo pèse ~{mb} Mo par joueur et par heure (estimation, mesurée dès les premières démos).', { mb: num(est.mb_per_hour, 0) })}</p>
          <p><b>{t('≈ {day} par jour → {keep} pour la durée choisie.', { day: gb(est.gb_per_day), keep: gb(est.keep_gb) })}</b></p>
          {#if est.source}
            <p>{est.source === 'disk' ? t('Place utilisable pour les démos : {s} (espace libre du disque, 20 % gardés en réserve).', { s: gb(est.space_gb) }) : t('Place réservée aux démos : {s}.', { s: gb(est.space_gb) })}
              {#if est.max_days}<b>{t('Maximum conseillé : {d}.', { d: days(est.max_days) })}</b>{/if}</p>
          {:else}
            <p class="muted">{t('L\'espace libre du serveur n\'est pas visible par FTP : indique la place que tu réserves aux démos (réglages avancés) pour avoir le maximum conseillé.')}</p>
          {/if}
          {#if est.over}<p class="warn">{t('La durée choisie dépasse la place disponible : les plus vieilles démos seront supprimées avant, dès que le disque est plein à 90 %.')}</p>{/if}
          <p class="muted small">{t('Calcul : joueurs en moyenne × heures par jour × Mo par joueur et par heure = place par jour ; place disponible ÷ place par jour = durée possible.')}</p>
        </div>
      {/if}

      <details>
        <summary>{t('Réglages avancés')}</summary>
        <div class="grid">
          <label class="fl"><span>{t('Dossier des démos (depuis le dossier des logs)')}</span><input bind:value={f.dir} placeholder={data.default_dir} autocapitalize="none" spellcheck="false" maxlength="300" /></label>
          {#if !data.local}<label class="fl"><span>{t('Place réservée aux démos (Go)')}</span><input type="number" min="0" step="0.5" bind:value={f.reserve_gb} /></label>{/if}
        </div>
      </details>

      <p class="muted small">{t('Pense à prévenir les joueurs, par exemple dans les règles ou le message du jour : « Les parties sont enregistrées pour l\'anti-triche ».')}</p>
      <button class="btn primary" disabled={busy}>{t('Enregistrer les réglages des démos')}</button>
    </form>
  {/if}
</section>

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; margin-top: 12px; }
  .box p { margin: 0; }
  form { display: flex; flex-direction: column; gap: 12px; }
  .small { font-size: 12px; }
  .switch { display: flex; gap: 10px; align-items: flex-start; font-size: 14px; }
  .switch input { width: 22px; height: 22px; flex: none; accent-color: var(--accent); margin-top: 1px; }
  .switch span { display: flex; flex-direction: column; gap: 2px; }
  .switch small { color: var(--muted); font-size: 12px; }
  .state { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; font-size: 13px; color: var(--text-2); }
  .state.on { color: var(--ok); }
  .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
  @media (max-width: 520px) { .grid { grid-template-columns: 1fr; } }
  .fl { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); min-width: 0; }
  .fl input, .keep select { height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; color: var(--text); min-width: 0; }
  .keep { display: flex; gap: 6px; }
  .keep input { flex: 1; min-width: 0; height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; color: var(--text); }
  .est { display: flex; flex-direction: column; gap: 6px; padding: 12px; border-radius: 12px; background: var(--surface-2); font-size: 13px; color: var(--text-2); }
  .est.over { box-shadow: inset 0 0 0 1px var(--warn); }
  .warn { color: var(--warn); font-weight: 700; }
  .bad { color: var(--danger-text); }
  details summary { cursor: pointer; font-weight: 800; }
  details .grid { margin-top: 10px; }
</style>
