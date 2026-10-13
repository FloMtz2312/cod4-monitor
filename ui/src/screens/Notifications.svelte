<script>
  // Notifications: the alerts this phone receives, even with the app closed (server down or back, report, suspect or
  // banned player back, B3 ban), among those the account has the rights to see; and the servers they come from.
  import BackTitle from '../components/BackTitle.svelte';
  import Icon from '../components/Icon.svelte';
  import { app, role } from '../lib/state.svelte.js';
  import { permsOf } from '../lib/perms.js';
  import { support, permission, ios, current, subscribe, save, unsubscribe, test } from '../lib/push.js';
  import { say } from '../lib/toast.svelte.js';
  import { href } from '../lib/router.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const DEFS = [
    { k: 'down', title: 'Serveur tombé ou revenu', sub: 'crash, redémarrage, retour en ligne', ok: p => p.size > 0 },
    { k: 'report', title: 'Nouveau signalement', sub: '!report tapé en jeu', ok: p => p.has('reports') || p.has('private') },
    { k: 'suspect', title: 'Joueur suspect ou banni qui revient', sub: 'stats anormales, joueur noté, même IP qu\'un banni', ok: p => p.has('private') },
    { k: 'b3ban', title: 'Ban B3', sub: 'ban ou ban temporaire donné en jeu', ok: p => p.has('private') }
  ];

  const visitor = $derived(role() === 'visitor');
  // the servers of the account, and what it may receive from at least one of them
  const servers = $derived(app.servers.filter(s => permsOf(app.me, s.id).size > 0));
  const kinds = $derived(DEFS.filter(d => servers.some(s => d.ok(permsOf(app.me, s.id)))));
  const sup = support();

  let perm = $state(permission());
  let cur = $state(null); // {json, sub} once read
  let busy = $state(false);
  let failed = $state('');

  async function load() {
    try {
      cur = await current();
      failed = '';
    } catch (e) {
      failed = e.message || 'error';
    }
  }
  $effect(() => { if (!visitor && sup === 'ok' && perm !== 'denied') load(); });

  async function enable() {
    busy = true;
    try {
      await subscribe(kinds.map(d => d.k), []);
      await load();
      say(t('Notifications activées sur ce téléphone'));
    } catch (e) {
      perm = permission();
      if (perm !== 'denied') say(e.message === 'off' ? t('Les notifications ne sont pas disponibles sur ce monitor.') : t('Échec : {err}', { err: e.message }));
    } finally {
      busy = false;
    }
  }

  async function update(types, muted) {
    const before = cur.sub;
    cur.sub = { ...before, types, muted };
    try {
      cur.sub = await save(cur.json, types, muted);
    } catch (e) {
      cur.sub = before;
      say(t('Échec : {err}', { err: e.message }));
    }
  }
  const toggle = (list, v) => (list.includes(v) ? list.filter(x => x !== v) : [...list, v]);

  async function sendTest() {
    busy = true;
    try {
      await test(cur.json.endpoint);
      say(t('Notification envoyée : elle arrive dans quelques secondes.'));
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
      if (e.status === 410) await load(); // forgotten by the push service: subscribe again
    } finally {
      busy = false;
    }
  }

  async function disable() {
    busy = true;
    try {
      await unsubscribe();
      await load();
      say(t('Notifications désactivées sur ce téléphone'));
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    } finally {
      busy = false;
    }
  }

  const clean = n => n.replace(/\^\d/g, '');
</script>

<BackTitle title={t('Notifications')} sub={t('Sur ce téléphone, même app fermée.')} />

{#if visitor}
  <div class="card box"><p class="muted">{t('Les notifications sont réservées à l\'équipe des serveurs : connecte-toi.')}</p>
    <a class="btn primary" href={href('connexion')}>{t('Se connecter')}</a></div>
{:else if !app.loaded}
  <p class="muted">{t('Chargement…')}</p>
{:else if !kinds.length}
  <div class="card box"><p class="muted">{t('Ton compte n\'a encore aucun droit sur un serveur : rien à recevoir pour l\'instant.')}</p></div>
{:else if sup === 'install'}
  <section class="card box">
    <h2>{t('Sur iPhone, installe d\'abord l\'app')}</h2>
    <p class="muted">{t('Apple n\'envoie les notifications qu\'aux sites ajoutés à l\'écran d\'accueil (iOS 16.4 ou plus récent).')}</p>
    <ol>
      <li>{t('Dans Safari, touche le bouton Partager.')}</li>
      <li>{t('Choisis « Sur l\'écran d\'accueil », puis Ajouter.')}</li>
      <li>{t('Ouvre CoD4 Monitor depuis l\'écran d\'accueil et reviens ici : Plus, puis Notifications.')}</li>
    </ol>
  </section>
{:else if sup === 'unsupported'}
  <div class="card box"><p class="muted">{t('Ce navigateur ne gère pas les notifications push. Essaie avec Chrome, Firefox, Edge ou Safari à jour.')}</p></div>
{:else if perm === 'denied'}
  <section class="card box">
    <h2>{t('Notifications bloquées')}</h2>
    <p class="muted">{ios()
      ? t('Autorise-les dans Réglages, Notifications, CoD4 Monitor, puis rouvre l\'app.')
      : t('Autorise-les pour ce site dans les réglages du navigateur (icône à gauche de l\'adresse), puis recharge la page.')}</p>
  </section>
{:else if failed}
  <div class="card box"><p class="muted">{t('Échec : {err}', { err: failed })}</p><button class="btn" onclick={load}>{t('Réessayer')}</button></div>
{:else if cur === null}
  <p class="muted">{t('Chargement…')}</p>
{:else if !cur.sub}
  <section class="card box intro">
    <span class="bell"><Icon name="bell" size={26} /></span>
    <h2>{t('Reçois les alertes sur ce téléphone')}</h2>
    <ul>
      {#each kinds as d (d.k)}<li><Icon name="check" size={16} />{t(d.title)}</li>{/each}
    </ul>
    <button class="btn primary" disabled={busy} onclick={enable}>{t('Activer les notifications')}</button>
    <p class="muted small">{t('Le téléphone va demander l\'autorisation. Tu pourras choisir ensuite ce que tu reçois.')}</p>
  </section>
{:else}
  <section>
    <h2 class="label sec">{t('Recevoir')}</h2>
    <div class="card rows">
      {#each kinds as d (d.k)}
        {@const on = cur.sub.types.includes(d.k)}
        <button class="row" role="switch" aria-checked={on} onclick={() => update(toggle(cur.sub.types, d.k), cur.sub.muted)}>
          <span class="rt"><b>{t(d.title)}</b><small>{t(d.sub)}</small></span><span class="sw" aria-hidden="true"></span>
        </button>
      {/each}
    </div>
  </section>

  {#if servers.length > 1}
    <section>
      <h2 class="label sec">{t('Serveurs')}</h2>
      <div class="card rows">
        {#each servers as s (s.id)}
          {@const on = !cur.sub.muted.includes(s.id)}
          <button class="row" role="switch" aria-checked={on} onclick={() => update(cur.sub.types, toggle(cur.sub.muted, s.id))}>
            <span class="rt"><b>{clean(s.name)}</b></span><span class="sw" aria-hidden="true"></span>
          </button>
        {/each}
      </div>
    </section>
  {/if}

  <div class="acts">
    <button class="btn" disabled={busy} onclick={sendTest}><Icon name="bell" size={18} />{t('Envoyer une notification de test')}</button>
    <button class="btn ghost" disabled={busy} onclick={disable}>{t('Désactiver sur ce téléphone')}</button>
  </div>
  <p class="muted small note">{t('Chaque téléphone ou navigateur se règle à part. Les alertes par mail ne changent pas.')}</p>
{/if}

<style>
  section { margin-top: 6px; }
  .sec { display: block; margin: 12px 2px 8px; }
  .box { padding: 16px; display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
  .box h2 { font-size: 16px; margin: 0; }
  .box p { margin: 0; font-size: 14px; }
  ol { margin: 0; padding-left: 20px; display: flex; flex-direction: column; gap: 6px; font-size: 14px; }
  .intro { align-items: stretch; }
  .bell { width: 48px; height: 48px; border-radius: 16px; background: var(--accent-soft); color: var(--accent); display: flex; align-items: center; justify-content: center; }
  .intro ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 6px; font-size: 14px; }
  .intro li { display: flex; gap: 8px; align-items: center; }
  .intro li :global(svg) { color: var(--ok); flex: none; }
  .small { font-size: 12px; }
  .rows { overflow: hidden; }
  .row {
    width: 100%; display: flex; align-items: center; gap: 12px; min-height: 60px; padding: 10px 16px; border: 0;
    border-top: 1px solid var(--line-soft); background: transparent; color: var(--text); text-align: left;
  }
  .row:first-child { border-top: 0; }
  .rt { flex: 1; min-width: 0; display: flex; flex-direction: column; gap: 2px; }
  .rt b { font-size: 15px; }
  .rt small { font-size: 12px; color: var(--muted); }
  .sw { flex: none; width: 46px; height: 28px; border-radius: 14px; background: var(--line); position: relative; transition: background .15s; }
  .sw::after { content: ''; position: absolute; top: 3px; left: 3px; width: 22px; height: 22px; border-radius: 11px; background: var(--text-2); transition: transform .15s; }
  .row[aria-checked='true'] .sw { background: var(--accent); }
  .row[aria-checked='true'] .sw::after { transform: translateX(18px); background: var(--on-accent); }
  .acts { display: flex; flex-direction: column; gap: 8px; margin-top: 16px; }
  .acts .btn { width: 100%; gap: 8px; }
  .note { margin: 12px 2px 0; }
</style>
