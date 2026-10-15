<script>
  // Old logs of a server (in its settings): upload an old games_mp.log or enterleave.log (the file kept aside when
  // the server started a new one, a backup…). The monitor imports it in the background while it keeps reading the
  // live logs; what it already has is skipped, so a file can be sent again.
  import Icon from './Icon.svelte';
  import { get } from '../lib/api.js';
  import { say } from '../lib/toast.svelte.js';
  import { size } from '../lib/demos.js';
  import { dt } from '../lib/moderation.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ serverId: number }} */
  let { serverId } = $props();

  let data = $state(null);
  let files = $state([]);
  let sending = $state(null); // { name, sent, total }

  function load() {
    return get(`/api/serverconf/${serverId}/imports`).then(r => { data = r; }).catch(() => {});
  }
  // refreshed while an import waits or runs
  $effect(() => {
    load();
    const timer = setInterval(() => {
      if ((data?.imports ?? []).some(j => j.state === 'pending' || j.state === 'running')) load();
    }, 3000);
    return () => clearInterval(timer);
  });

  function upload(file) {
    return new Promise((resolve, reject) => {
      const x = new XMLHttpRequest();
      x.open('PUT', `/api/serverconf/${serverId}/imports?name=${encodeURIComponent(file.name)}`);
      x.setRequestHeader('Content-Type', 'application/octet-stream');
      x.upload.onprogress = e => { sending = { name: file.name, sent: e.loaded, total: e.total }; };
      x.onload = () => {
        let msg = '';
        try { msg = JSON.parse(x.responseText).error ?? ''; } catch {}
        x.status >= 200 && x.status < 300 ? resolve() : reject(new Error(msg || `HTTP ${x.status}`));
      };
      x.onerror = () => reject(new Error(t('connexion perdue')));
      x.send(file);
    });
  }

  async function send(e) {
    e.preventDefault();
    for (const file of files) {
      if (data?.max_bytes && file.size > data.max_bytes) {
        say(t('{f} : fichier trop gros ({max} au plus)', { f: file.name, max: size(data.max_bytes) }));
        continue;
      }
      try {
        sending = { name: file.name, sent: 0, total: file.size };
        await upload(file);
        say(t('{f} envoyé, import en cours', { f: file.name }));
      } catch (err) {
        say(t('{f} : échec de l\'envoi ({err})', { f: file.name, err: err.message }));
      }
    }
    sending = null;
    files = [];
    e.target.reset();
    load();
  }

  const KIND = { enterleave: 'connexions', games: 'parties' };
  const STATE = { pending: 'en attente', running: 'en cours', done: 'terminé', failed: 'échec' };
</script>

<section class="card box">
  <h2 class="label">{t('Importer d\'anciens logs')}</h2>
  <p class="muted small">{t('Pour récupérer les données d\'un ancien games_mp.log ou enterleave.log : le fichier mis de côté quand le serveur en a commencé un nouveau, une sauvegarde… Le monitor l\'importe en arrière-plan sans arrêter la lecture en direct, et ignore ce qu\'il a déjà.')}</p>
  <p class="muted small">{t('Envoie d\'abord l\'enterleave.log : il sert à dater les lignes du games_mp.log de la même période.')}</p>
  {#if data && !data.enabled}
    <p class="muted small">{t('Import indisponible sur ce monitor.')}</p>
  {:else}
    <form onsubmit={send}>
      <input type="file" multiple accept=".log,.txt,.old,text/plain" onchange={e => (files = [...e.currentTarget.files])} aria-label={t('Fichiers de log')} />
      <button class="btn primary" disabled={!files.length || sending}><Icon name="download" size={18} />{t('Envoyer')}</button>
    </form>
    {#if sending}
      <div class="bar" role="progressbar" aria-valuemin="0" aria-valuemax={sending.total} aria-valuenow={sending.sent} aria-label={t('Envoi de {f}', { f: sending.name })}>
        <span style:width="{sending.total ? Math.round(100 * sending.sent / sending.total) : 0}%"></span>
      </div>
      <p class="muted small">{t('Envoi de {f} : {a} / {b}', { f: sending.name, a: size(sending.sent), b: size(sending.total) })}</p>
    {/if}
    {#if data?.imports?.length}
      <div class="jobs">
        {#each data.imports as j (j.id)}
          <div class="job" class:bad={j.state === 'failed'}>
            <span class="top"><b>{j.file_name}</b><span class="st {j.state}">{t(STATE[j.state] ?? j.state)}</span></span>
            <small class="muted">{dt(j.created_at)} · {size(j.size)}{#if j.kind}{' · '}{t(KIND[j.kind] ?? j.kind)}{/if}</small>
            {#if j.lines}
              <small>{t('{l} lignes lues · {i} importées · {s} déjà connues', { l: j.lines, i: j.imported, s: j.skipped })}{#if j.undated}{' · '}{t('{u} sans date', { u: j.undated })}{/if}</small>
            {/if}
            {#if j.error}<small class="err">{j.error}</small>{/if}
          </div>
        {/each}
      </div>
    {/if}
  {/if}
</section>

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; margin-top: 12px; }
  .box p { margin: 0; }
  .small { font-size: 12px; }
  form { display: flex; flex-wrap: wrap; gap: 8px; align-items: center; }
  form input { flex: 1 1 220px; min-width: 0; font-size: 14px; color: var(--text-2); }
  .bar { height: 8px; border-radius: 4px; background: var(--surface-2); overflow: hidden; }
  .bar span { display: block; height: 100%; background: var(--accent); }
  .jobs { display: flex; flex-direction: column; }
  .job { display: flex; flex-direction: column; gap: 2px; padding: 8px 0; border-top: 1px solid var(--line-soft); font-size: 13px; }
  .job:first-child { border-top: 0; }
  .top { display: flex; justify-content: space-between; gap: 8px; }
  .top b { overflow-wrap: anywhere; }
  .st { font-size: 12px; font-weight: 800; white-space: nowrap; color: var(--text-2); }
  .st.running, .st.pending { color: var(--accent); }
  .st.done { color: var(--ok); }
  .st.failed, .err { color: var(--danger-text); }
</style>
