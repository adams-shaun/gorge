<script lang="ts">
  import { manaSymbols } from '../lib/mana';

  /** Renders a Forge-notation mana cost as small pips; unrecognised notation renders nothing rather than guessing. */
  let { cost }: { cost: string } = $props();

  const symbols = $derived(manaSymbols(cost));
</script>

{#if symbols.length}
  <span class="mana-symbols">
    {#each symbols as s, i (i)}<span class="pip">{s}</span>{/each}
  </span>
{/if}

<style>
  .mana-symbols { display: inline-flex; gap: .15em; vertical-align: middle; }
  .pip {
    display: inline-flex; align-items: center; justify-content: center;
    min-width: 1.1em; height: 1.1em; padding: 0 .15em;
    border-radius: 999px; background: #55585f; color: #fff;
    font-size: .65em; font-weight: 700; line-height: 1;
  }
</style>
