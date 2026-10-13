<script>
  // A player name with its in-game colors (^1, ^2…).
  import { colorParts } from '../lib/format.js';

  /** @type {{ raw: string }} */
  let { raw } = $props();
  const parts = $derived(colorParts(raw));
</script>

<span class="name">{#each parts as p, i (i)}<span style:color={p.color}>{p.text}</span>{/each}</span>

<style>
  .name { white-space: nowrap; overflow: hidden; text-overflow: ellipsis; }
</style>
