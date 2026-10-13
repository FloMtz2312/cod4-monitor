<script>
  // Réglages d'un serveur (#/reglages/<id>, #/reglages/nouveau): address, rcon, where the logs are read (FTP or a
  // folder of this machine), advanced settings; "Tester" checks everything before saving. Passwords are never
  // sent back: left empty, the saved ones are kept.
  import BackTitle from '../components/BackTitle.svelte';
  import { api, get, post } from '../lib/api.js';
  import { app, loadServers, selectServer } from '../lib/state.svelte.js';
  import { route, back } from '../lib/router.svelte.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const param = $derived(route.params[0] ?? 'nouveau');
  const isNew = $derived(param === 'nouveau');
  const superAdmin = $derived(!!(app.me?.admin || app.me?.noauth));

  let conf = $state(null); // saved settings (view), null while loading
  let roots = $state({ local: '', b3: '' });
  let missing = $state(false);
  let f = $state(blank());
  let source = $state('none'); // none | ftp | local
  let b3kind = $state('none'); // none | mysql | sqlite
  let test = $state(null);
  let busy = $state('');

  function blank() {
    return { name: '', address: '', rcon: '', rcon_clear: false, ftp_tls: false, ftp_host: '', ftp_port: 21, ftp_user: '', ftp_pass: '',
      ftp_path: '/mods/', log_dir: '', games_log: '', console_log: '', enterleave_log: '', log_tz: '', b3_db: '', enabled: true,
      b3_host: '', b3_port: 3306, b3_user: '', b3_pass: '', b3_name: 'b3', b3_tls: false };
  }

  $effect(() => {
    const p = param;
    test = null;
    missing = false;
    get('/api/serverconf').then(r => {
      roots = { local: r.local_root ?? '', b3: r.b3_root ?? '' };
      if (p === 'nouveau') { conf = {}; f = blank(); source = 'none'; b3kind = 'none'; return; }
      const c = (r.servers ?? []).find(s => String(s.id) === p);
      if (!c) { missing = true; return; }
      conf = c;
      f = { ...blank(), name: c.name, address: c.address, enabled: c.enabled, games_log: c.games_log ?? '', console_log: c.console_log ?? '',
        enterleave_log: c.enterleave_log ?? '', log_tz: c.log_tz ?? '', b3_db: c.b3_db ?? '', log_dir: c.log_dir ?? '',
        ...(c.ftp ? { ftp_tls: c.ftp.tls, ftp_host: c.ftp.host, ftp_port: c.ftp.port, ftp_user: c.ftp.user, ftp_path: c.ftp.path } : {}),
        ...(c.b3_mysql ? { b3_host: c.b3_mysql.host, b3_port: c.b3_mysql.port, b3_user: c.b3_mysql.user, b3_name: c.b3_mysql.name, b3_tls: c.b3_mysql.tls } : {}) };
      source = c.log_dir ? 'local' : c.ftp ? 'ftp' : 'none';
      b3kind = c.b3_mysql ? 'mysql' : c.b3_db ? 'sqlite' : 'none';
    }).catch(e => { missing = true; say(e.message); });
  });

  // what the server expects: the unused log source is cleared
  const body = () => {
    const { b3_host, b3_port, b3_user, b3_pass, b3_name, b3_tls, ...rest } = f;
    return {
      ...rest, ftp_port: Number(f.ftp_port) || 21,
      ftp_clear: source !== 'ftp', log_dir: source === 'local' ? f.log_dir : '',
      ftp_host: source === 'ftp' ? f.ftp_host : '',
      b3_db: b3kind === 'sqlite' ? f.b3_db : '',
      b3_mysql: b3kind === 'mysql' ? { host: b3_host.trim(), port: Number(b3_port) || 3306, user: b3_user.trim(), pass: b3_pass, name: b3_name.trim(), tls: b3_tls } : null,
      b3_clear: b3kind === 'none'
    };
  };

  async function runTest() {
    busy = 'test';
    test = null;
    try {
      test = await post('/api/servertest', { ...body(), id: isNew ? 0 : conf.id });
    } catch (e) {
      test = { error: e.message };
    } finally {
      busy = '';
    }
  }

  async function save(e) {
    e.preventDefault();
    busy = 'save';
    try {
      const r = isNew ? await post('/api/serverconf', body()) : await api('PUT', `/api/serverconf/${conf.id}`, body());
      await loadServers();
      if (r.warning) say(r.warning); else say(t('Serveur enregistré'));
      if (isNew && r.id) selectServer(r.id);
      back('serveurs');
    } catch (err) {
      say(t('Échec : {err}', { err: err.message }));
    } finally {
      busy = '';
    }
  }

  async function setEnabled(on) {
    busy = 'enabled';
    try {
      const r = await post(`/api/serverconf/${conf.id}/enabled`, { enabled: on });
      f.enabled = on;
      await loadServers();
      say(r.warning ?? (on ? t('Surveillance activée') : t('Surveillance arrêtée (l\'historique est gardé)')));
    } catch (err) {
      say(t('Échec : {err}', { err: err.message }));
    } finally {
      busy = '';
    }
  }

  // transfer to another administrator account (super admin): clears the team and the invitations of the server
  let admins = $state(null);
  let newOwner = $state('');
  let sureTransfer = $state(false);
  $effect(() => {
    if (!superAdmin || isNew) return;
    get('/api/users').then(r => { admins = (r.users ?? []).filter(u => u.account_role === 'admin').map(u => u.username); }).catch(() => { admins = []; });
  });
  $effect(() => { newOwner = conf?.owner ?? ''; });

  async function transfer() {
    if (!sureTransfer) { sureTransfer = true; return; }
    busy = 'owner';
    try {
      await api('PUT', `/api/servers/${conf.id}/owner`, { username: newOwner });
      conf.owner = newOwner;
      await loadServers();
      say(t('Serveur transféré'));
    } catch (err) {
      say(t('Échec : {err}', { err: t(err.message) }));
    } finally {
      busy = '';
      sureTransfer = false;
    }
  }

  const checks = $derived(test && !test.error ? [
    ['query', t('Le serveur répond'), test.query && (test.query.ok ? `${test.query.hostname ?? ''} · ${test.query.map ?? ''} · ${test.query.players}/${test.query.max}` : test.query.error)],
    ['rcon', t('Mot de passe rcon'), test.rcon && (test.rcon.ok ? t('accepté') : test.rcon.error)],
    ['ftp', t('Logs'), test.ftp && (test.ftp.ok ? t('trouvés : {f}', { f: (test.ftp.found ?? []).join(', ') }) : test.ftp.error + (test.ftp.mods?.length ? ` · ${t('mods :')} ${test.ftp.mods.join(', ')}` : ''))],
    ['b3', 'B3', test.b3 && (test.b3.ok ? t('{a} admins, {p} sanctions', { a: test.b3.admins, p: test.b3.penalties }) : test.b3.error)]
  ].filter(([k]) => test[k]) : []);
</script>

<BackTitle title={isNew ? t('Ajouter un serveur') : t('Réglages du serveur')} sub={!isNew && conf?.name ? conf.name.replace(/\^\d/g, '') : ''} fallback="serveurs" />

{#if missing}
  <p class="muted">{t('Serveur introuvable, ou réservé à son propriétaire.')}</p>
{:else if conf === null}
  <p class="muted">{t('Chargement…')}</p>
{:else}
  <form onsubmit={save}>
    <section class="card box">
      <h2 class="label">{t('Serveur de jeu')}</h2>
      <label class="fl"><span>{t('Nom')}</span><input bind:value={f.name} required maxlength="80" /></label>
      <label class="fl"><span>{t('Adresse (ip:port)')}</span><input bind:value={f.address} required placeholder="203.0.113.10:28960" autocapitalize="none" spellcheck="false" /></label>
      <label class="fl"><span>{t('Mot de passe rcon')}</span>
        <input type="password" bind:value={f.rcon} autocomplete="new-password" disabled={f.rcon_clear}
          placeholder={conf.has_rcon ? t('enregistré : laisser vide pour le garder') : t('facultatif : actions en jeu, chat, commandes')} />
      </label>
      {#if conf.has_rcon}
        <label class="check"><input type="checkbox" bind:checked={f.rcon_clear} />{t('Supprimer le mot de passe rcon')}</label>
      {/if}
    </section>

    <section class="card box">
      <h2 class="label">{t('Logs du serveur')}</h2>
      <p class="muted small">{t('Les logs donnent les kills, le chat, les rounds et les connexions. Sans eux : seulement l\'état et les joueurs connectés.')}</p>
      <div class="opts" role="radiogroup" aria-label={t('Source des logs')}>
        <button type="button" role="radio" aria-checked={source === 'none'} onclick={() => (source = 'none')}>{t('Pas de logs')}</button>
        <button type="button" role="radio" aria-checked={source === 'ftp'} onclick={() => (source = 'ftp')}>FTP</button>
        {#if superAdmin && roots.local}<button type="button" role="radio" aria-checked={source === 'local'} onclick={() => (source = 'local')}>{t('Sur cette machine')}</button>{/if}
      </div>
      {#if source === 'ftp'}
        <div class="grid">
          <label class="fl wide"><span>{t('Hôte FTP')}</span><input bind:value={f.ftp_host} required placeholder="ftp.mon-hebergeur.fr" autocapitalize="none" spellcheck="false" /></label>
          <label class="fl"><span>{t('Port')}</span><input type="number" min="1" max="65535" bind:value={f.ftp_port} /></label>
          <label class="fl"><span>{t('Utilisateur')}</span><input bind:value={f.ftp_user} autocomplete="off" autocapitalize="none" spellcheck="false" /></label>
          <label class="fl"><span>{t('Mot de passe FTP')}</span><input type="password" bind:value={f.ftp_pass} autocomplete="new-password" placeholder={conf.ftp?.has_pass ? t('enregistré') : ''} /></label>
          <label class="fl wide"><span>{t('Dossier des logs')}</span><input bind:value={f.ftp_path} placeholder="/mods/mon_mod/" autocapitalize="none" spellcheck="false" /></label>
        </div>
        <label class="check"><input type="checkbox" bind:checked={f.ftp_tls} />{t('Connexion chiffrée (FTPS)')}</label>
      {:else if source === 'local'}
        <label class="fl"><span>{t('Dossier (dans {root})', { root: roots.local })}</span><input bind:value={f.log_dir} required placeholder={`${roots.local}/cod4/mods/mon_mod/`} autocapitalize="none" spellcheck="false" /></label>
      {/if}
    </section>

    <section class="card box">
      <h2 class="label">{t('B3 (bot d\'administration)')}</h2>
      <p class="muted small">{t('Si B3 tourne sur ce serveur : ses sanctions, admins et grades dans le site, alertes de ban, actions B3 depuis la fiche des joueurs.')}</p>
      <div class="opts" role="radiogroup" aria-label={t('Base de données de B3')}>
        <button type="button" role="radio" aria-checked={b3kind === 'none'} onclick={() => (b3kind = 'none')}>{t('Pas de B3')}</button>
        <button type="button" role="radio" aria-checked={b3kind === 'mysql'} onclick={() => (b3kind = 'mysql')}>MySQL / MariaDB</button>
        {#if superAdmin && roots.b3}<button type="button" role="radio" aria-checked={b3kind === 'sqlite'} onclick={() => (b3kind = 'sqlite')}>{t('Fichier SQLite sur cette machine')}</button>{/if}
      </div>
      {#if b3kind === 'mysql'}
        <p class="muted small">{t('La base indiquée dans le b3.ini de B3 (storage: mysql://…). Le serveur MySQL doit accepter les connexions venant du monitor ; le compte doit pouvoir lire et écrire la base de B3.')}</p>
        <div class="grid">
          <label class="fl wide"><span>{t('Hôte MySQL')}</span><input bind:value={f.b3_host} required placeholder="db.mon-hebergeur.fr" autocapitalize="none" spellcheck="false" /></label>
          {#if superAdmin && roots.b3}<p class="muted small wide">{t('MariaDB sur cette machine (projet cod4-server) : mets le chemin de son socket, {path}.', { path: `${roots.b3.replace(/\/$/, '')}/mysqld/mysqld.sock` })}</p>{/if}
          <label class="fl"><span>{t('Port')}</span><input type="number" min="1" max="65535" bind:value={f.b3_port} /></label>
          <label class="fl"><span>{t('Base de données')}</span><input bind:value={f.b3_name} required placeholder="b3" autocapitalize="none" spellcheck="false" /></label>
          <label class="fl"><span>{t('Utilisateur')}</span><input bind:value={f.b3_user} required autocomplete="off" autocapitalize="none" spellcheck="false" /></label>
          <label class="fl"><span>{t('Mot de passe MySQL')}</span><input type="password" bind:value={f.b3_pass} autocomplete="new-password" placeholder={conf.b3_mysql?.has_pass ? t('enregistré') : ''} /></label>
        </div>
        <label class="check"><input type="checkbox" bind:checked={f.b3_tls} />{t('Connexion chiffrée (TLS)')}</label>
      {:else if b3kind === 'sqlite'}
        <label class="fl"><span>{t('Base B3 (dans {root})', { root: roots.b3 })}</span><input bind:value={f.b3_db} required placeholder={`${roots.b3}/b3.db`} autocapitalize="none" spellcheck="false" /></label>
      {/if}
    </section>

    <details class="card box adv">
      <summary>{t('Réglages avancés')}</summary>
      <p class="muted small">{t('À remplir seulement si les logs ont d\'autres noms, ou si l\'heure du serveur n\'est pas celle du monitor.')}</p>
      <div class="grid">
        <label class="fl"><span>games_mp.log</span><input bind:value={f.games_log} placeholder="games_mp.log" autocapitalize="none" spellcheck="false" /></label>
        <label class="fl"><span>console_mp.log</span><input bind:value={f.console_log} placeholder="console_mp.log" autocapitalize="none" spellcheck="false" /></label>
        <label class="fl"><span>enterleave.log</span><input bind:value={f.enterleave_log} placeholder="enterleave.log" autocapitalize="none" spellcheck="false" /></label>
        <label class="fl"><span>{t('Fuseau horaire du serveur')}</span><input bind:value={f.log_tz} placeholder="Europe/Paris" autocapitalize="none" spellcheck="false" /></label>
      </div>
    </details>

    {#if test}
      <section class="card box" aria-live="polite">
        <h2 class="label">{t('Test')}</h2>
        {#if test.error}<p class="bad">{test.error}</p>{/if}
        {#each checks as [k, label, detail] (k)}
          <p class="chk"><span class="ic" class:ok={test[k].ok}>{test[k].ok ? '✓' : '✕'}</span><span><b>{label}</b><span class="muted">{detail}</span></span></p>
        {/each}
      </section>
    {/if}

    <div class="acts">
      <button type="button" class="btn" disabled={!!busy} onclick={runTest}>{busy === 'test' ? t('Test en cours…') : t('Tester')}</button>
      <button class="btn primary" disabled={!!busy}>{t('Enregistrer')}</button>
    </div>
  </form>

  {#if !isNew}
    <section class="card box">
      <h2 class="label">{t('Surveillance')}</h2>
      {#if f.enabled}
        <p class="muted small">{t('Le monitor interroge ce serveur et lit ses logs.')}</p>
        <button class="btn ghost danger-t" disabled={!!busy} onclick={() => setEnabled(false)}>{t('Arrêter la surveillance')}</button>
      {:else}
        <p class="muted small">{t('Surveillance arrêtée : l\'historique est gardé.')}</p>
        <button class="btn" disabled={!!busy} onclick={() => setEnabled(true)}>{t('Reprendre la surveillance')}</button>
      {/if}
    </section>

    {#if superAdmin && admins}
      <section class="card box">
        <h2 class="label">{t('Propriétaire')}</h2>
        <p class="muted small">{t('Actuel : {who}. Le transfert retire l\'équipe et les invitations de ce serveur.', { who: conf.owner || t('Super admin (.env)') })}</p>
        <div class="own">
          <select bind:value={newOwner} aria-label={t('Nouveau propriétaire')} onchange={() => (sureTransfer = false)}>
            <option value="">{t('Super admin (.env)')}</option>
            {#each admins as u (u)}<option value={u}>{u}</option>{/each}
          </select>
          <button class="btn" class:danger={sureTransfer} disabled={!!busy || newOwner === (conf.owner ?? '')} onclick={transfer}>{sureTransfer ? t('Confirmer le transfert') : t('Transférer')}</button>
        </div>
      </section>
    {/if}
  {/if}
{/if}

<style>
  .own { display: flex; flex-wrap: wrap; gap: 8px; }
  .own select { flex: 1 1 200px; min-width: 0; height: 46px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 10px; font-size: 16px; }
  form { display: flex; flex-direction: column; gap: 12px; }
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 12px; }
  section.box:last-child { margin-top: 12px; }
  .box p { margin: 0; }
  .small { font-size: 12px; }
  .fl { display: flex; flex-direction: column; gap: 6px; font-size: 13px; font-weight: 700; color: var(--text-2); min-width: 0; }
  .fl input { height: 44px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 12px; font-size: 16px; color: var(--text); min-width: 0; }
  .fl input:disabled { opacity: .5; }
  .grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 10px; }
  .wide { grid-column: 1 / -1; }
  .check { display: flex; align-items: center; gap: 8px; font-size: 14px; color: var(--text-2); }
  .check input { width: 20px; height: 20px; accent-color: var(--accent); }
  .opts { display: flex; flex-wrap: wrap; gap: 6px; }
  .opts button { height: 38px; padding: 0 12px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text-2); font-weight: 700; }
  .opts button[aria-checked='true'] { border-color: var(--accent); background: var(--accent-soft); color: var(--accent); }
  .adv summary { cursor: pointer; font-weight: 800; }
  .chk { display: flex; gap: 10px; align-items: flex-start; font-size: 14px; }
  .chk > span:last-child { display: flex; flex-direction: column; min-width: 0; overflow-wrap: anywhere; }
  .ic { width: 22px; height: 22px; flex: none; border-radius: 7px; display: flex; align-items: center; justify-content: center; font-weight: 800; background: rgba(255, 99, 105, .14); color: var(--danger-text); }
  .ic.ok { background: rgba(61, 214, 140, .14); color: var(--ok); }
  .bad { color: var(--danger-text); }
  .acts { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .danger-t { color: var(--danger-text); border-color: #5A2A30; }
</style>
