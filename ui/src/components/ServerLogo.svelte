<script>
  // Logo of a server (the owner's picture), else its initials on the accent color.
  import { initials } from '../lib/state.svelte.js';
  import { logoUrl } from '../lib/brand.js';

  /** @type {{ server: any, size?: number, muted?: boolean }} */
  let { server, size = 36, muted = false } = $props();
  let broken = $state(false);
  const src = $derived(logoUrl(server));
  $effect(() => { src; broken = false; });
</script>

<span class="logo" class:muted style:width={`${size}px`} style:height={`${size}px`} style:font-size={`${Math.round(size * 0.38)}px`}
  style:background={!muted && server?.accent ? server.accent : null}>
  {#if src && !broken}<img {src} alt="" onerror={() => (broken = true)} />{:else}{server ? initials(server.name) : 'C4'}{/if}
</span>

<style>
  .logo {
    flex: none; border-radius: 28%; background: var(--accent); color: var(--on-accent); overflow: hidden;
    display: flex; align-items: center; justify-content: center; font-family: var(--font-display); font-weight: 700;
  }
  .logo.muted { background: #33405A; color: var(--text); }
  img { width: 100%; height: 100%; object-fit: cover; display: block; }
</style>
