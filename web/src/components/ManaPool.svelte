<script lang="ts">
  /**
   * ManaPool draws one player's floating mana. The wire's pool is exactly the
   * six symbols `poolView` emits (view/visibility.go), so this reads them in a
   * fixed WUBRGC order rather than iterating the object: a readout that
   * reorders itself between frames cannot be read at a glance, and object key
   * order is not a guarantee worth depending on.
   *
   * An empty pool renders NOTHING — not an empty row. Floating mana matters
   * precisely because it is unusual, and a permanent empty slot teaches the
   * eye to skip the one place where the unusual thing will appear.
   *
   * Colour carries the identity and the mono figure carries the amount: this
   * is the instrument register, so the symbol is a plain colour chip rather
   * than a card-face pip, and the count sits beside it in the data face where
   * every other number in this client lives.
   */
  let { pool }: { pool: Record<string, number> } = $props();

  const ORDER = ['W', 'U', 'B', 'R', 'G', 'C'] as const;
  const NAMES: Record<string, string> = {
    W: 'white', U: 'blue', B: 'black', R: 'red', G: 'green', C: 'colourless',
  };

  const held = $derived(
    ORDER.map((sym) => ({ sym, n: pool[sym] ?? 0 })).filter((h) => h.n > 0),
  );
  const summary = $derived(held.map((h) => `${h.n} ${NAMES[h.sym]}`).join(', '));
</script>

{#if held.length > 0}
  <div class="mana-pool" data-mana-pool aria-label="Mana pool: {summary}">
    {#each held as h (h.sym)}
      <span class="sym" data-mana={h.sym} title="{h.n} {NAMES[h.sym]}">
        <span class="chip" style="--pip: var(--mana-{h.sym.toLowerCase()})" aria-hidden="true"></span>
        <span class="n">{h.n}</span>
      </span>
    {/each}
  </div>
{/if}

<style>
  .mana-pool {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
  }
  .sym {
    display: inline-flex;
    align-items: center;
    gap: 0.25em;
  }
  /* Saturation means mana here, which is exactly what the design system
     reserves it for; the ring keeps a near-white W chip from dissolving into
     the panel it sits on. */
  .chip {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 999px;
    background: var(--pip);
    box-shadow: inset 0 0 0 1px rgb(0 0 0 / 0.35);
    flex: none;
  }
  .n {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: var(--t-12);
    line-height: 1;
    color: var(--ink-inst);
  }
</style>
