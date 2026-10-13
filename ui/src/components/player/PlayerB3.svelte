<script>
  // B3 block of a profile (staff with the private data): grade, mask, greeting, connections, penalties with their end,
  // author and origin, B3's names / IPs / linked accounts. The owner can change the grade, lift bans and penalties, sanction.
  import CodName from '../CodName.svelte';
  import B3PenaltySheet from '../B3PenaltySheet.svelte';
  import { get, post } from '../../lib/api.js';
  import { href } from '../../lib/router.svelte.js';
  import { clean } from '../../lib/format.js';
  import { dt, day } from '../../lib/dates.js';
  import { say } from '../../lib/toast.svelte.js';
  import { t } from '../../lib/i18n.svelte.js';

  /** @type {{ serverId: number, guid: string, nameRaw: string, owner: boolean, online: boolean }} */
  let { serverId, guid, nameRaw, owner, online } = $props();

  let b3 = $state(null);
  let off = $state(false);
  let sheet = $state(false);
  let busy = $state(false);
  let grade = $state('');
  /** @type {null | {kind: string, id?: number, group?: any}} */
  let ask = $state(null);

  function load() {
    get(`/api/servers/${serverId}/b3/player?guid=${encodeURIComponent(guid)}`)
      .then(r => { b3 = r; grade = r.client ? String(r.client.group?.id ?? '') : ''; })
      .catch(() => { off = true; });
  }
  $effect(() => {
    serverId; guid;
    b3 = null;
    off = false;
    load();
  });

  const KIND = { Warning: 'Avertissement', Kick: 'Kick', TempBan: 'Ban temporaire', Ban: 'Ban', Notice: 'Note B3' };
  const c = $derived(b3?.client ?? null);
  const pens = $derived(b3?.penalties ?? []);
  const warns = $derived(pens.filter(x => x.active && x.type === 'Warning').length);
  const banned = $derived(pens.some(x => x.active && (x.type === 'Ban' || x.type === 'TempBan')));
  const groups = $derived(b3?.groups ?? []);
  const now = Date.now() / 1000;
  const end = x => x.type === 'Kick' || x.type === 'Notice' ? '—'
    : x.inactive ? t('levé')
    : x.time_expire === -1 ? t('jamais')
    : x.time_expire < now ? t('expiré') : dt(x.time_expire);
  const by = x => x.origin === 'auto' ? t('B3 (auto)') : x.origin === 'site' ? x.admin : t('{a} (en jeu)', { a: x.admin });

  async function call(fn, done) {
    busy = true;
    try {
      const r = await fn();
      say(done(r));
      load();
    } catch (e) {
      say(t('Échec : {err}', { err: e.message }));
    } finally {
      busy = false;
      ask = null;
    }
  }
  const unban = () => call(() => post(`/api/servers/${serverId}/b3/unban`, { guid, name: nameRaw }), () => t('Ban levé : {name}', { name: clean(nameRaw) }));
  const lift = id => call(() => post(`/api/servers/${serverId}/b3/penalty/${id}/lift`), () => t('Sanction retirée'));
  const setGroup = g => call(() => post(`/api/servers/${serverId}/b3/group`, { guid, name: nameRaw, group: g.id }),
    r => t('{name} : grade {g}', { name: clean(nameRaw), g: r.group?.name ?? g.name }) + (r.online ? ` (${t('actif à sa prochaine connexion')})` : ''));

  function pickGrade() {
    const g = groups.find(x => String(x.id) === grade);
    if (g && g.id !== c?.group?.id) ask = { kind: 'group', group: g };
  }
  function cancel() {
    ask = null;
    grade = c ? String(c.group?.id ?? '') : '';
  }
</script>

{#if !off}
  <section class="card box" aria-label="B3">
    <div class="hd">
      <h2 class="label">B3</h2>
      {#if c}<span class="grade">{c.group?.name ?? 'Guest'} · {c.group?.level ?? 0}</span>{:else if b3}<span class="muted small">{t('jamais vu par B3')}</span>{/if}
    </div>
    {#if !b3}
      <p class="muted">{t('Chargement…')}</p>
    {:else}
      {#if c}
        <div class="facts">
          {#if c.mask}<span>{t('Affiché aux joueurs comme {g} (!mask)', { g: c.mask.name })}</span>{/if}
          {#if c.greeting}<span>{t('Message d\'accueil B3 :')} <CodName raw={c.greeting} /></span>{/if}
          <span>{t('{n} connexion(s) · vu par B3 depuis le {d}', { n: c.connections, d: day(c.time_add) })}</span>
          {#if warns || banned}
            <span>{#if warns}<b class="warn">{t('{n} avertissement(s) en cours', { n: warns })}</b>{/if}{#if warns && banned}{' · '}{/if}{#if banned}<b class="ko">{t('banni')}</b>{/if}</span>
          {/if}
        </div>
      {/if}

      {#if owner}
        <div class="acts">
          <button class="btn danger" onclick={() => (sheet = true)}>{t('Sanctionner')}</button>
          {#if banned}<button class="btn" disabled={busy} onclick={() => (ask = { kind: 'unban' })}>{t('Lever le ban')}</button>{/if}
          {#if c && groups.length}
            <label class="grade-pick">
              <span>{t('Grade B3')}</span>
              <select bind:value={grade} onchange={pickGrade} disabled={busy}>
                {#each groups as g (g.id)}<option value={String(g.id)}>{g.name} · {g.level}</option>{/each}
              </select>
            </label>
          {/if}
        </div>
        {#if ask}
          <div class="ask" role="alertdialog" aria-label={t('Confirmation')}>
            <span>
              {#if ask.kind === 'unban'}{t('Lever le ban de {name} ?', { name: clean(nameRaw) })}
              {:else if ask.kind === 'lift'}{t('Retirer cette sanction ?')}
              {:else}{t('Passer {name} au grade {g} ({l}) ?', { name: clean(nameRaw), g: ask.group.name, l: ask.group.level })}{/if}
            </span>
            <div>
              <button class="btn ghost" onclick={cancel}>{t('Annuler')}</button>
              <button class="btn primary" disabled={busy}
                onclick={() => (ask.kind === 'unban' ? unban() : ask.kind === 'lift' ? lift(ask.id) : setGroup(ask.group))}>{t('Confirmer')}</button>
            </div>
          </div>
        {/if}
      {/if}

      <h3 class="label sub">{t('Sanctions')}</h3>
      {#each pens.slice(0, 50) as x (x.id)}
        <div class="pen">
          <div class="l1">
            <span class="kind" class:active={x.active} class:ban={x.type === 'Ban' || x.type === 'TempBan'}>{t(KIND[x.type] ?? x.type)}</span>
            <span class="why">{x.reason || '—'}{#if x.keyword}<span class="kw">#{x.keyword}</span>{/if}</span>
            {#if owner && x.active}
              {#if x.type === 'Ban' || x.type === 'TempBan'}
                <button class="mini" disabled={busy} onclick={() => (ask = { kind: 'unban' })}>{t('Lever')}</button>
              {:else}
                <button class="mini" disabled={busy} onclick={() => (ask = { kind: 'lift', id: x.id })}>{t('Retirer')}</button>
              {/if}
            {/if}
          </div>
          <div class="l2 muted">{dt(x.time_add)}{' · '}{t('par {a}', { a: by(x) })}{' · '}{t('fin : {e}', { e: end(x) })}</div>
        </div>
      {:else}
        <p class="muted">{t('Aucune sanction.')}</p>
      {/each}

      {#if c}
        <div class="grid">
          <div>
            <h3 class="label sub">{t('Pseudos (B3)')}</h3>
            {#each b3.names ?? [] as n, i (i)}<div class="li"><span class="v"><CodName raw={n.value} /></span><span class="muted">{n.used}× · {day(n.last)}</span></div>{:else}<p class="muted">—</p>{/each}
          </div>
          <div>
            <h3 class="label sub">{t('IP (B3)')}</h3>
            {#each b3.ips ?? [] as n, i (i)}<div class="li"><span class="v mono">{n.value}</span><span class="muted">{n.used}× · {day(n.last)}</span></div>{:else}<p class="muted">—</p>{/each}
          </div>
          <div>
            <h3 class="label sub">{t('Comptes liés par IP (B3)')}</h3>
            {#each b3.linked ?? [] as l (l.guid)}
              <div class="li">
                <a class="v" href={href('joueur', l.guid)}><CodName raw={l.name} />{#if l.banned}<span class="kind ban active">{t('banni')}</span>{/if}</a>
                <span class="muted">{(l.shared ?? []).join(', ')} · {day(l.time_edit)}</span>
              </div>
            {:else}<p class="muted">{t('Aucun')}</p>{/each}
          </div>
        </div>
      {/if}
    {/if}
  </section>
  {#if owner}
    <B3PenaltySheet open={sheet} onclose={() => (sheet = false)} {serverId} {guid} {nameRaw} {online} ondone={load} />
  {/if}
{/if}

<style>
  .box { padding: 14px 16px; display: flex; flex-direction: column; gap: 10px; }
  .box p { margin: 0; }
  .hd { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
  .grade { font-size: 13px; font-weight: 800; color: var(--accent); }
  .small { font-size: 12px; }
  .facts { display: flex; flex-direction: column; gap: 4px; font-size: 13px; color: var(--text-2); }
  .facts span { display: flex; flex-wrap: wrap; gap: 4px; }
  .warn { color: var(--warn); }
  .ko { color: var(--danger-text); }
  .acts { display: flex; flex-wrap: wrap; gap: 8px; align-items: flex-end; }
  .grade-pick { display: flex; flex-direction: column; gap: 4px; font-size: 12px; font-weight: 700; color: var(--muted); flex: 1 1 180px; max-width: 360px; }
  .grade-pick select { height: 46px; border-radius: 14px; border: 1px solid var(--line); background: var(--surface-2); padding: 0 10px; font-size: 16px; }
  .ask { display: flex; flex-direction: column; gap: 8px; padding: 12px; border-radius: 14px; border: 1px solid var(--accent); background: var(--accent-soft); font-size: 14px; font-weight: 700; }
  .ask div { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .sub { margin-top: 6px; }
  .pen { display: flex; flex-direction: column; gap: 3px; padding: 8px 0; border-top: 1px solid var(--line-soft); }
  .l1 { display: flex; align-items: center; gap: 8px; font-size: 14px; min-width: 0; }
  .l2 { font-size: 12px; }
  .kind { flex: none; font-size: 12px; font-weight: 800; padding: 3px 8px; border-radius: 8px; background: var(--surface-2); color: var(--text-2); }
  .kind.active { background: rgba(245, 196, 81, .15); color: var(--warn); }
  .kind.ban.active { background: rgba(255, 99, 105, .14); color: var(--danger-text); }
  .why { flex: 1; min-width: 0; overflow-wrap: anywhere; }
  .kw { margin-left: 6px; font-size: 12px; color: var(--muted); }
  .mini { flex: none; height: 30px; padding: 0 10px; border-radius: 9px; border: 1px solid var(--line); background: var(--surface-2); font-size: 12px; font-weight: 800; }
  .grid { display: grid; grid-template-columns: minmax(0, 1fr); gap: 4px 16px; }
  .li { display: flex; justify-content: space-between; gap: 8px; padding: 5px 0; font-size: 13px; border-top: 1px solid var(--line-soft); }
  .li .v { min-width: 0; display: flex; align-items: center; gap: 6px; font-weight: 700; color: var(--text); text-decoration: none; overflow: hidden; }
  .li .muted { flex: none; font-size: 12px; text-align: right; max-width: 55%; overflow-wrap: anywhere; }
  .mono { font-family: ui-monospace, monospace; }
  @media (min-width: 960px) { .grid { grid-template-columns: repeat(3, minmax(0, 1fr)); } }
</style>
