<script lang="ts">
  /**
   * ManaPool draws the seat box's line-3 mana readout, which the product spec
   * (B1) defines as "the mana pool OR free mana one gains by tapping lands or
   * other effects". Those are two different things and the box has one row, so
   * this renders BOTH, deliberately distinct:
   *
   *   - AVAILABLE (the outlined chips) is what player could produce right now
   *     by tapping untapped permanents' free-to-tap mana abilities. It is
   *     derived from the public battlefield, so it is present on EVERY seat
   *     and EVERY visibility — and it is the content of this row in the
   *     ordinary case, because floating mana only ever exists inside a step.
   *   - POOL (the solid saturated chips) is mana already floating. It is a
   *     seat-private value (view.PlayerView.Pool is filled only for the
   *     viewer's own seat and deliberately carries no omitempty, so a hidden
   *     pool arrives as a literal JSON null).
   *
   * A reader must never mistake the two, so they are separated by a divider
   * and styled opposite ways: available mana is hollow (a potential, not yet
   * spent), floating mana is saturated (mana here). When either group is
   * empty it renders nothing, and when both are empty the row still exists
   * (IdentityBar reserves its height) so the box does not grow a line when
   * mana appears and collapse the moment it is spent.
   *
   * The wire's two maps are exactly the six symbols poolView emits
   * (view/visibility.go), so this reads them in a fixed WUBRGC order rather
   * than iterating the object: a readout that reorders itself between frames
   * cannot be read at a glance, and object key order is not a guarantee worth
   * depending on.
   *
   * `available` is public (always a non-nil object from the server), but the
   * generated protocol.ts types both fields as plain Records, so neither
   * svelte-check nor the type checker can catch a caller that hands this a
   * null — the null/absent guard therefore lives here, at the one place that
   * owns "an absent group draws nothing".
   */
  let { pool, available }: {
    pool: Record<string, number> | null | undefined;
    available?: Record<string, number> | null | undefined;
  } = $props();

  const ORDER = ['W', 'U', 'B', 'R', 'G', 'C'] as const;
  const NAMES: Record<string, string> = {
    W: 'white', U: 'blue', B: 'black', R: 'red', G: 'green', C: 'colourless',
  };

  const held = $derived(
    ORDER.map((sym) => ({ sym, n: pool?.[sym] ?? 0 })).filter((h) => h.n > 0),
  );
  const avail = $derived(
    ORDER.map((sym) => ({ sym, n: available?.[sym] ?? 0 })).filter((h) => h.n > 0),
  );
  const poolSummary = $derived(held.map((h) => `${h.n} ${NAMES[h.sym]}`).join(', '));
  const availSummary = $derived(avail.map((h) => `${h.n} ${NAMES[h.sym]}`).join(', '));
</script>

{#if avail.length > 0 || held.length > 0}
  <div class="readout" data-mana-readout>
    {#if avail.length > 0}
      <div class="group avail" data-mana-available aria-label="Available by tapping: {availSummary}">
        {#each avail as h (h.sym)}
          <span class="sym" data-avail={h.sym} title="{h.n} {NAMES[h.sym]} available">
            <span class="chip outline" style="--pip: var(--mana-{h.sym.toLowerCase()})" aria-hidden="true"></span>
            <span class="n">{h.n}</span>
          </span>
        {/each}
      </div>
    {/if}
    {#if avail.length > 0 && held.length > 0}
      <span class="sep" data-mana-sep aria-hidden="true"></span>
    {/if}
    {#if held.length > 0}
      <div class="group pool" data-mana-pool aria-label="Mana pool: {poolSummary}">
        {#each held as h (h.sym)}
          <span class="sym" data-mana={h.sym} title="{h.n} {NAMES[h.sym]}">
            <span class="chip" style="--pip: var(--mana-{h.sym.toLowerCase()})" aria-hidden="true"></span>
            <span class="n">{h.n}</span>
          </span>
        {/each}
      </div>
    {/if}
  </div>
{/if}

<style>
  .readout {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--sp-2);
    min-width: 0;
  }
  .group {
    display: flex;
    align-items: center;
    gap: var(--sp-2);
  }
  /* The two groups are deliberately styled opposite ways so a reader cannot
     mistake mana that COULD be tapped (hollow) for mana already floating
     (solid). The divider between them when both are present is the only
     textual separator a reader needs. */
  .sep {
    width: 1px;
    align-self: stretch;
    background: var(--edge-felt);
    flex: none;
  }
  .sym {
    display: inline-flex;
    align-items: center;
    gap: 0.25em;
  }
  /* Saturation means mana already floating, which is exactly what the design
     system reserves it for; the ring keeps a near-white W chip from
     dissolving into the panel it sits on. */
  .chip {
    width: 0.6rem;
    height: 0.6rem;
    border-radius: 999px;
    background: var(--pip);
    box-shadow: inset 0 0 0 1px rgb(0 0 0 / 0.35);
    flex: none;
  }
  /* Available mana is a potential, not yet spent: hollow (a ring in the
     colour's hue, transparent interior) rather than a full saturated fill,
     so it reads as "could tap" against the pool's "already here". */
  .chip.outline {
    background: transparent;
    box-shadow: inset 0 0 0 1.5px var(--pip);
  }
  .n {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: var(--t-12);
    line-height: 1;
    color: var(--ink-inst);
  }
</style>
