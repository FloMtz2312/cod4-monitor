<script>
  // #/moi/<code>: the link given by !me in game. Pins the profile ("C'est moi") on this phone and opens it.
  import { onMount } from 'svelte';
  import { app, selectServer } from '../lib/state.svelte.js';
  import { claim } from '../lib/me.svelte.js';
  import { route, replace, href } from '../lib/router.svelte.js';
  import { clean } from '../lib/format.js';
  import { say } from '../lib/toast.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  let error = $state('');

  onMount(async () => {
    try {
      const r = await claim(route.params[0] ?? '');
      if (app.servers.some(s => s.id === r.server_id) || !app.loaded) selectServer(r.server_id);
      say(t('C\'est toi : {name} · tes stats sont épinglées', { name: clean(r.name_raw) }));
      replace('joueur', r.key);
    } catch (e) {
      error = e.message;
    }
  });
</script>

{#if error}
  <section class="card box">
    <h1>{t('Code !me')}</h1>
    <p class="muted">{error}</p>
    <a class="btn" href={href('joueurs')}>{t('Chercher mon pseudo')}</a>
  </section>
{:else}
  <p class="muted">{t('Chargement…')}</p>
{/if}

<style>
  .box { padding: 16px; display: flex; flex-direction: column; gap: 10px; align-items: flex-start; }
  h1 { font-size: 20px; }
  p { margin: 0; }
</style>
