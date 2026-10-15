<script>
  // Admins' chat of the server: for the accounts of its staff whose player in game is a B3 admin (level 20 or more).
  // The account is linked once to its player with a code typed in game (!link <code>). @12 mentions the admin with
  // the B3 id 12, @name by account or player name, @all everyone: the admin gets a push notification and a badge.
  import { untrack } from 'svelte';
  import BackTitle from '../components/BackTitle.svelte';
  import CodName from '../components/CodName.svelte';
  import Icon from '../components/Icon.svelte';
  import { api, get, post } from '../lib/api.js';
  import { app, currentServer } from '../lib/state.svelte.js';
  import { loadAdminChat } from '../lib/adminchat.svelte.js';
  import { clean } from '../lib/format.js';
  import { dt } from '../lib/moderation.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  const id = $derived(app.serverId);
  const srv = $derived(currentServer());

  let info = $state(null); // GET /adminchat without the messages
  let error = $state('');
  let messages = $state([]);
  let older = $state(false); // older messages may exist
  let code = $state(null); // { code, until }
  let text = $state('');
  let sending = $state(false);
  let box = $state();
  let list = $state();
  let now = $state(Date.now());

  const lastId = () => (messages.length ? messages[messages.length - 1].id : 0);

  function atBottom() {
    return !list || list.scrollHeight - list.scrollTop - list.clientHeight < 80;
  }
  function toBottom() {
    requestAnimationFrame(() => { if (list) list.scrollTop = list.scrollHeight; });
  }

  async function markRead() {
    if (!messages.length || document.hidden) return;
    await post(`/api/servers/${id}/adminchat/read`, { id: lastId() }).catch(() => {});
    loadAdminChat(id);
  }

  async function load(sid, fresh) {
    try {
      const after = info?.admin ? lastId() : 0;
      const r = await get(`/api/servers/${sid}/adminchat?${new URLSearchParams({ after: String(after), fresh: fresh ? '1' : '' })}`);
      if (sid !== id) return;
      const wasAdmin = info?.admin;
      const { messages: ms, ...rest } = r;
      info = rest;
      error = '';
      if (!r.admin) {
        messages = [];
        return;
      }
      if (r.linked) code = null;
      if (!wasAdmin || !after) {
        messages = ms ?? [];
        older = messages.length >= 100;
        toBottom();
        markRead();
      } else if (ms?.length) {
        const stick = atBottom();
        messages = [...messages, ...ms.filter(m => m.id > after)];
        if (stick) toBottom();
        markRead();
      }
    } catch (e) {
      if (sid === id) error = e.message;
    }
  }

  $effect(() => {
    const sid = id;
    if (!sid) return;
    untrack(() => {
      info = null;
      messages = [];
      code = null;
      load(sid, true);
    });
    // new messages every 5 s on screen; every 3 s while waiting for !link in game
    const every = setInterval(() => {
      now = Date.now();
      if (!document.hidden) load(sid, !!code);
    }, 4000);
    return () => clearInterval(every);
  });

  async function more() {
    const r = await get(`/api/servers/${id}/adminchat?before=${messages[0].id}`).catch(() => null);
    if (!r?.messages) return;
    const h = list?.scrollHeight ?? 0;
    messages = [...r.messages, ...messages];
    older = r.messages.length >= 100;
    requestAnimationFrame(() => { if (list) list.scrollTop += list.scrollHeight - h; });
  }

  async function askCode() {
    try {
      const r = await post(`/api/servers/${id}/adminchat/link`);
      code = { code: r.code, until: Date.now() + r.ttl_s * 1000 };
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    }
  }

  async function unlink() {
    try {
      await api('DELETE', `/api/servers/${id}/adminchat/link`, {});
      info = null;
      messages = [];
      await load(id, true);
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    }
  }

  async function send() {
    const body = text.trim();
    if (!body || sending) return;
    sending = true;
    try {
      const r = await post(`/api/servers/${id}/adminchat`, { body });
      text = '';
      if (!messages.some(m => m.id === r.message.id)) messages = [...messages, r.message];
      toBottom();
      if (r.message.mentions?.length) say(t(r.message.mentions.length > 1 ? '{n} admins notifiés' : '{n} admin notifié', { n: r.message.mentions.length }));
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    } finally {
      sending = false;
      box?.focus();
    }
  }

  function keydown(e) {
    if (e.key === 'Enter' && !e.shiftKey && !e.isComposing) {
      e.preventDefault();
      send();
    }
  }

  // the @… being typed before the cursor: suggestions of admins
  let caret = $state(0);
  const partial = $derived.by(() => {
    const m = /(^|[^\p{L}\p{N}_])@([\p{L}\p{N}_.-]*)$/u.exec(text.slice(0, caret));
    return m ? m[2].toLowerCase() : null;
  });
  const others = $derived((info?.admins ?? []).filter(a => a.username !== info?.me));
  const suggest = $derived(partial === null ? [] : others.filter(a => !partial || String(a.b3_id).startsWith(partial)
    || a.username.toLowerCase().startsWith(partial) || clean(a.name).toLowerCase().replace(/\s+/g, '').startsWith(partial)));

  function mention(a) {
    const before = text.slice(0, caret), after = text.slice(caret);
    const start = partial === null ? before.length : before.length - partial.length - 1;
    const ins = (start > 0 && !/\s$/.test(before.slice(0, start)) ? ' ' : '') + '@' + a.b3_id + ' ';
    text = before.slice(0, start) + ins + after;
    caret = start + ins.length;
    requestAnimationFrame(() => { box?.focus(); box?.setSelectionRange(caret, caret); });
  }
  const track = e => { caret = e.target.selectionStart ?? text.length; };

  // the text of a message, with its @ mentions shown (mine stand out)
  const myTags = $derived(info?.self ? ['@' + info.self.b3_id, '@' + info.self.username.toLowerCase(), '@' + clean(info.self.name).toLowerCase().replace(/\s+/g, ''), '@all', '@tous', '@alle'] : []);
  function parts(body) {
    const out = [];
    let last = 0;
    for (const m of body.matchAll(/(^|[^\p{L}\p{N}_])(@[\p{L}\p{N}_.-]{1,32})/gu)) {
      const at = m.index + m[1].length;
      out.push({ text: body.slice(last, at) });
      const tag = m[2].replace(/[.-]+$/, '');
      out.push({ text: tag, at: true, me: myTags.includes(tag.toLowerCase()) });
      last = at + tag.length;
    }
    out.push({ text: body.slice(last) });
    return out;
  }
  const mine = m => m.username === info?.me;
  const left = $derived(code ? Math.max(0, Math.ceil((code.until - now) / 60000)) : 0);
</script>

<BackTitle title={t('Tchat des admins')} sub={srv ? clean(srv.name) : ''} fallback="moderation" />

{#if error}
  <p class="muted">{error}</p>
{:else if info === null}
  <p class="muted">{t('Chargement…')}</p>
{:else if !info.b3}
  <section class="card box">
    <p>{t('Le tchat des admins utilise les grades de B3 : configure la base B3 de ce serveur (Réglages du serveur → réglages avancés).')}</p>
  </section>
{:else if !info.linked}
  <section class="card box">
    <h2>{t('Relie ton compte à ton joueur')}</h2>
    <p class="muted">{t('Le tchat est réservé aux admins B3 de niveau {n} ou plus. Pour savoir quel joueur tu es, tape un code en jeu, une seule fois.', { n: info.level_min })}</p>
    {#if code && left > 0}
      <p class="cmd">!link <b>{code.code}</b></p>
      <p class="muted">{t('Tape cette commande dans le chat du jeu sur ce serveur (valable {n} min). Cette page se met à jour toute seule.', { n: left })}</p>
    {:else}
      <button class="btn primary" onclick={askCode}><Icon name="link" size={18} />{t('Obtenir un code')}</button>
    {/if}
  </section>
{:else if !info.admin}
  <section class="card box">
    <h2>{t('Accès réservé aux admins')}</h2>
    <p class="muted">{t('Ton compte est relié à {name}, qui n\'est pas admin B3 de niveau {n} ou plus sur ce serveur.', { name: clean(info.player) || '—', n: info.level_min })}</p>
    <div class="acts">
      <button class="btn" onclick={askCode}>{t('Relier un autre joueur')}</button>
      <button class="btn" onclick={unlink}>{t('Délier')}</button>
    </div>
    {#if code && left > 0}
      <p class="cmd">!link <b>{code.code}</b></p>
      <p class="muted">{t('Tape cette commande dans le chat du jeu sur ce serveur (valable {n} min). Cette page se met à jour toute seule.', { n: left })}</p>
    {/if}
  </section>
{:else}
  <div class="admins" role="group" aria-label={t('Admins du tchat')}>
    {#each info.admins as a (a.username)}
      <button class="adm" class:me={a.username === info.me} disabled={a.username === info.me} onclick={() => mention(a)} title={t('Mentionner {name}', { name: clean(a.name) })}>
        <span class="id">@{a.b3_id}</span><CodName raw={a.name} /><span class="muted lvl">{a.group}</span>
      </button>
    {/each}
  </div>

  <section class="card chat">
    <div class="list" bind:this={list}>
      {#if older}<button class="link oldb" onclick={more}>{t('Messages précédents')}</button>{/if}
      {#each messages as m (m.id)}
        <div class="msg" class:mine={mine(m)} class:hit={!mine(m) && m.mentions?.includes(info.me)}>
          <div class="meta">
            <span class="id">@{m.b3_id}</span>
            <CodName raw={m.name_raw || m.username} />
            <span class="muted when">{dt(m.ts)}</span>
          </div>
          <p>{#each parts(m.body) as p, j (j)}{#if p.at}<span class="tag" class:me={p.me}>{p.text}</span>{:else}{p.text}{/if}{/each}</p>
        </div>
      {:else}
        <p class="muted empty">{t('Pas encore de message. Écris @ suivi du numéro B3 d\'un admin pour le prévenir.')}</p>
      {/each}
    </div>

    {#if suggest.length}
      <div class="suggest" role="listbox" aria-label={t('Mentionner')}>
        {#each suggest.slice(0, 6) as a (a.username)}
          <button role="option" aria-selected="false" onmousedown={e => e.preventDefault()} onclick={() => mention(a)}><span class="id">@{a.b3_id}</span><CodName raw={a.name} /></button>
        {/each}
      </div>
    {/if}
    <form class="write" onsubmit={e => { e.preventDefault(); send(); }}>
      <label class="sr-only" for="ac-text">{t('Message')}</label>
      <textarea id="ac-text" bind:this={box} bind:value={text} rows="1" maxlength="1000" placeholder={t('Message aux admins… @12 pour mentionner')}
        onkeydown={keydown} onkeyup={track} onclick={track} oninput={track}></textarea>
      <button class="btn primary" type="submit" disabled={sending || !text.trim()} aria-label={t('Envoyer')}><Icon name="send" size={18} /></button>
    </form>
  </section>
  <p class="muted foot">{t('Relié à {name}.', { name: clean(info.player) })} <button class="link" onclick={unlink}>{t('Délier')}</button></p>
{/if}

<style>
  .box { padding: 16px; display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
  .box p, .box h2 { margin: 0; }
  .box h2 { font-size: 16px; font-weight: 800; }
  .cmd { font-family: var(--font-mono, monospace); font-size: 22px; padding: 10px 14px; border-radius: 12px; background: var(--surface-2); user-select: all; }
  .cmd b { color: var(--accent); letter-spacing: 2px; }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; }
  .admins { display: flex; gap: 6px; overflow-x: auto; margin-bottom: 10px; padding-bottom: 2px; scrollbar-width: none; }
  .adm { flex: none; display: inline-flex; align-items: center; gap: 6px; height: 38px; padding: 0 12px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface); color: var(--text); font-weight: 700; font-size: 13px; }
  .adm.me { opacity: .6; }
  .lvl { font-size: 11px; font-weight: 600; }
  .id { font-size: 11px; font-weight: 800; padding: 1px 6px; border-radius: 6px; background: var(--accent-soft); color: var(--accent); flex: none; }
  .chat { display: flex; flex-direction: column; overflow: hidden; }
  .list { height: max(220px, calc(100dvh - 450px - env(safe-area-inset-bottom))); overflow-y: auto; padding: 8px 0; display: flex; flex-direction: column; }
  .oldb { align-self: center; margin: 4px 0 8px; }
  .msg { padding: 8px 16px; min-width: 0; }
  .msg.hit { background: var(--accent-soft); }
  .meta { display: flex; align-items: center; gap: 8px; min-width: 0; font-size: 13px; font-weight: 700; }
  .when { margin-left: auto; font-size: 12px; font-weight: 500; white-space: nowrap; }
  .msg p { margin: 3px 0 0; font-size: 15px; color: var(--text); white-space: pre-wrap; overflow-wrap: anywhere; }
  .msg.mine p { color: var(--text-2); }
  .tag { color: var(--accent); font-weight: 700; }
  .tag.me { background: var(--accent); color: #fff; border-radius: 5px; padding: 0 3px; }
  .empty { margin: auto 16px; text-align: center; }
  .suggest { display: flex; gap: 6px; overflow-x: auto; padding: 8px 12px 0; border-top: 1px solid var(--line-soft); scrollbar-width: none; }
  .suggest button { flex: none; display: inline-flex; align-items: center; gap: 6px; height: 36px; padding: 0 10px; border-radius: 10px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text); font-weight: 700; font-size: 13px; }
  .write { display: flex; gap: 8px; padding: 10px 12px; border-top: 1px solid var(--line-soft); align-items: flex-end; }
  .suggest + .write { border-top: 0; }
  textarea { flex: 1; min-width: 0; min-height: 44px; max-height: 140px; resize: none; field-sizing: content; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); color: var(--text); padding: 11px 12px; font: inherit; font-size: 16px; }
  .write .btn { width: 48px; height: 44px; padding: 0; flex: none; }
  .foot { font-size: 13px; margin: 10px 0 0; display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
  @media (min-width: 960px) { .list { height: max(300px, calc(100dvh - 330px)); } }
  .link { border: 0; background: none; color: var(--accent); font-weight: 800; font-size: 13px; padding: 0; }
</style>
