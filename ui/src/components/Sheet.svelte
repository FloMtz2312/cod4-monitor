<script>
  // Bottom sheet on a phone, centered box on a large screen. Native <dialog>: Escape and focus are handled by the browser.
  /** @type {{ open: boolean, onclose: () => void, label: string, children: import('svelte').Snippet }} */
  let { open, onclose, label, children } = $props();
  /** @type {HTMLDialogElement} */
  let dlg;

  $effect(() => {
    if (!dlg) return;
    if (open && !dlg.open) dlg.showModal();
    else if (!open && dlg.open) dlg.close();
  });
</script>

<!-- A tap on the backdrop (the dialog itself, outside its content) closes it; Escape is native. -->
<!-- svelte-ignore a11y_click_events_have_key_events, a11y_no_noninteractive_element_interactions -->
<dialog bind:this={dlg} aria-label={label} {onclose} onclick={e => { if (e.target === dlg) dlg.close(); }}>
  <div class="content">
    <div class="handle" aria-hidden="true"></div>
    {@render children()}
  </div>
</dialog>

<style>
  dialog {
    position: fixed; inset: auto 0 0 0; margin: 0 auto; width: 100%; max-width: 560px; max-height: min(82dvh, calc(100dvh - var(--sat) - 12px));
    padding: 0; border: 1px solid #2A3446; border-bottom: 0; border-radius: 26px 26px 0 0;
    background: var(--surface); color: var(--text); overflow-y: auto;
  }
  dialog::backdrop { background: rgba(5, 7, 10, .66); }
  .content { padding: 10px 16px calc(28px + env(safe-area-inset-bottom)); }
  .handle { width: 40px; height: 5px; border-radius: 3px; background: #2E3A4D; margin: 0 auto 14px; }
  @media (min-width: 960px) {
    dialog { inset: 0; margin: auto; border-radius: 22px; border-bottom: 1px solid #2A3446; max-height: 80dvh; }
    .handle { display: none; }
  }
</style>
