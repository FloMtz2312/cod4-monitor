<script>
  import { icons } from '../lib/icons.js';

  /** @type {{ name: string, size?: number, stroke?: number }} */
  let { name, size = 22, stroke = 1.8 } = $props();
</script>

<svg viewBox="0 0 24 24" width={size} height={size} aria-hidden="true" fill="none" stroke="currentColor"
  stroke-width={stroke} stroke-linecap="round" stroke-linejoin="round">
  {#each icons[name] ?? [] as [tag, attrs]}
    <svelte:element this={tag} xmlns="http://www.w3.org/2000/svg" {...attrs} />
  {/each}
</svg>
