<script>
  // Small search field that filters a list in place.
  /** @type {{ value: string, label: string }} */
  let { value = $bindable(), label } = $props();
</script>

<input class="filter" type="search" bind:value placeholder={label} aria-label={label} autocomplete="off" spellcheck="false" />

<style>
  .filter { width: 100%; max-width: 260px; height: 38px; padding: 0 12px; border-radius: 12px; border: 1px solid var(--line); background: var(--surface-2); font-size: 16px; color: var(--text); }
</style>
