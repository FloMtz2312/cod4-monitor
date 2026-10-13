<script>
  import { toast } from '../lib/toast.svelte.js';
</script>

<div class="toast" role="status" aria-live="polite">
  {#if toast.msg}<span>{toast.msg}</span>{/if}
</div>

<style>
  .toast { position: fixed; left: 0; right: 0; bottom: calc(var(--nav-h) + 30px + env(safe-area-inset-bottom)); z-index: 40; display: flex; justify-content: center; pointer-events: none; padding: 0 24px; }
  span { padding: 12px 16px; border-radius: 14px; background: var(--text); color: var(--on-accent); font-weight: 700; font-size: 14px; box-shadow: 0 8px 24px rgba(0, 0, 0, .4); text-align: center; }
  @media (min-width: 960px) { .toast { bottom: 32px; padding-left: 264px; } }
</style>
