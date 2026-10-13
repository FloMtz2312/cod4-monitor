<script>
  // Header of a screen opened from another one: "Retour", the title and an optional line under it.
  import Icon from './Icon.svelte';
  import { back } from '../lib/router.svelte.js';
  import { t } from '../lib/i18n.svelte.js';

  /** @type {{ title: string, sub?: string, fallback?: string }} */
  let { title, sub = '', fallback = 'plus' } = $props();
</script>

<button class="back" onclick={() => back(fallback)}><Icon name="back" />{t('Retour')}</button>
<h1 class="title">{title}</h1>
{#if sub}<p class="sub muted">{sub}</p>{/if}

<style>
  .back { display: inline-flex; align-items: center; gap: 4px; height: 40px; padding: 0 10px 0 4px; margin: -4px 0 8px -4px; border: 0; background: transparent; color: var(--text-2); font-weight: 700; }
  .title { margin-bottom: 4px; }
  .sub { margin: 0 0 14px; font-size: 14px; }
  .title:last-child { margin-bottom: 14px; }
</style>
