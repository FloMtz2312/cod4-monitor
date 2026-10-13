<script>
  // Segmented control (tabs of a screen, periods): one choice among a few.
  /** @type {{ items: [string, string][], value: string, onchange: (v: string) => void, label: string }} */
  let { items, value, onchange, label } = $props();
</script>

<div class="seg" role="tablist" aria-label={label}>
  {#each items as [k, text] (k)}
    <button role="tab" aria-selected={value === k} class:on={value === k} onclick={() => onchange(k)}>{text}</button>
  {/each}
</div>

<style>
  .seg { display: flex; gap: 4px; padding: 4px; background: var(--surface); border: 1px solid var(--line); border-radius: 14px; }
  .seg button { flex: 1 1 auto; min-width: 0; height: 38px; padding: 0 6px; border: 0; border-radius: 10px; background: transparent; color: var(--text-2); font-weight: 700; font-size: 13px; white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
  .seg button.on { background: var(--accent); color: var(--on-accent); }
</style>
