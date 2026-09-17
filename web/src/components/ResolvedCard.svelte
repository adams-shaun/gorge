<script lang="ts">
  import type { View } from '../protocol';
  import { recentlyMattered, findCardAnywhere } from '../lib/board';
  import CardImage from './CardImage.svelte';

  /**
   * ResolvedCard is the stack section's own record of the last resolved
   * object (task fb-20260916T225456Z): a spell resolving is a card popping
   * off THIS stack, so its artwork shows here — a labelled row above the
   * tiles still on the stack — where it reads as the stack's history rather
   * than a poster parked over the combat seam. It replaces the old
   * RecentStrip overlay, which sat absolutely positioned over the board's
   * bottom centre and was itself a patchwork of two earlier complaints
   * about the same element (survey #26's lift, then a 220px→132px resize).
   *
   * Lifetime: the card persists until the NEXT resolve replaces it. The
   * wall-clock expiry the overlay carried (RECENT_MS = 1000ms) existed only
   * because the overlay parked over the board and occluded play; inside the
   * rail's own scroll region it occludes nothing, so the timer is gone and
   * the display is derived purely from the event list. recentlyMattered's
   * RECENT_RESOLVE_WINDOW event bound still applies: with no further
   * resolve within that window (roughly half a player turn, measured) the
   * row clears itself rather than lingering all game.
   *
   * The data path is unchanged from the strip: recentlyMattered finds the
   * most recent stack_resolve, and the resolved object has already moved to
   * graveyard/battlefield/exile (or back to a hand) by the time the view
   * renders, so findCardAnywhere locates it there. Derived values render
   * identically under SSR, which is what the component tests exercise.
   */
  let { view, events }: { view: View; events: { event: { kind: string; obj?: number } }[] } = $props();

  const card = $derived.by(() => {
    const id = recentlyMattered(events);
    return id === null ? null : findCardAnywhere(view, id);
  });
</script>

{#if card}
  <div class="resolved" data-resolved={card.id}>
    <span class="resolved__label">just resolved</span>
    <CardImage {card} />
  </div>
{/if}

<style>
  /* Instrument register, not the felt overlay box that was here before: the
     rail's own tokens — a raised cool ground, a hairline edge, the data
     face for the label — and the stack tile's 56px art width, so the
     resolved card reads as an entry OF this stack, not a visitor from the
     board. Art at tile size keeps the row to the stack section's density
     contract (U-rail-2); the large read stays one hover away elsewhere. */
  .resolved {
    display: flex;
    align-items: flex-start;
    gap: 0.5rem;
    padding: 0.3rem 0.4rem;
    margin-bottom: 0.3rem;
    background: var(--instrument-raised);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius-card);
    --card-w: 56px;
  }
  .resolved__label {
    flex: 1;
    min-width: 0;
    padding-top: 0.2rem;
    font-family: var(--font-data);
    font-size: var(--t-10);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--ink-faint);
  }
</style>
