<script lang="ts">
  import type { View, SeatInfo } from '../protocol';
  import { recentlyMattered, findCardAnywhere } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import CardImage from './CardImage.svelte';
  import CardDetail from './CardDetail.svelte';
  import { HoverCard, type AnchorRect } from '../lib/carddetail.svelte';

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
   *
   * NAME + HOVER + OWNER (fb-20260917T131253Z-41d199c8): the row read as
   * "just resolved" + bare art — no name in text (only the img alt), no
   * hover detail, no notion of who resolved it. Now it renders the card's
   * real name as text beside the label, wires the same HoverCard
   * dwell/focus/CardDetail inspector every other card surface uses
   * (StackTile, CardTile, HandList — ResolvedCard was the one surface
   * without it), and colours the row's border by the resolving seat.
   *
   * Ownership reads `card.controller`, not `card.owner`: for a just
   * resolved spell the question a reader of the stack's history asks is
   * WHO resolved it, and the two diverge only after a control-change
   * effect — at which point controller is the seat whose spell it was. The
   * colour source is the shared `seatColour` (server-known colour wins,
   * palette fallback), the same legend every seat-coloured element uses;
   * the hover panel's ledger footer carries the seat number too.
   *
   * The hover lifecycle mirrors StackTile: hover/anchor are injectable so
   * the SSR test harness can drive them, and the `$effect` supervise uses
   * a list derived from the CURRENT view (the card this row is showing, or
   * nothing) — when a newer resolve replaces this one while the panel is
   * open, supervise closes it because the old id is no longer what the
   * row is showing.
   */
  let {
    view,
    events,
    seats = undefined,
    hover = new HoverCard(),
    anchor: anchorProp = null,
  }: {
    view: View;
    events: { event: { kind: string; obj?: number } }[];
    /** the live match's seat list for seatColour's server-known colours; optional — the palette fallback works without it. */
    seats?: SeatInfo[];
    hover?: HoverCard;
    anchor?: AnchorRect | null;
  } = $props();

  const card = $derived.by(() => {
    const id = recentlyMattered(events);
    return id === null ? null : findCardAnywhere(view, id);
  });

  const ownerColour = $derived(card ? seatColour(card.controller, seats) : '');

  let root = $state<HTMLElement | null>(null);
  // Seeded from the prop (a test's injected rect) at mount/SSR time only —
  // in production only capture() ever sets it, from a real pointer event.
  let anchor = $state<AnchorRect | null>(anchorProp ?? null);

  $effect(() => {
    if (card) hover.supervise(card.id, [card]);
  });

  function capture(): void {
    const r = root?.getBoundingClientRect();
    anchor = r ? { left: r.left, top: r.top, right: r.right } : null;
  }
</script>

{#if card}
  <div class="resolved" data-resolved={card.id} style:border-color={ownerColour}>
    <div class="resolved__meta">
      <span class="resolved__label">just resolved</span>
      <span class="resolved__name">{card.name}</span>
    </div>
    <div
      class="resolved__art"
      bind:this={root}
      tabindex="0"
      role="button"
      aria-label={card.name}
      onpointerenter={() => hover.arm(card.id, capture)}
      onpointerleave={() => hover.close()}
      onfocus={() => hover.open(card.id, capture)}
      onblur={() => hover.close()}
      onkeydown={(e) => hover.keydown(e)}
      aria-describedby={hover.show ? `card-detail-${card.id}` : undefined}
    >
      <CardImage {card} />
    </div>
    {#if hover.show && anchor}<CardDetail {card} {anchor} />{/if}
  </div>
{/if}

<style>
  /* Instrument register, not the felt overlay box that was here before: the
     rail's own tokens — a raised cool ground, a hairline edge, the data
     face for the label — and the stack tile's 56px art width, so the
     resolved card reads as an entry OF this stack, not a visitor from the
     board. Art at tile size keeps the row to the stack section's density
     contract (U-rail-2); the large read stays one hover away. The border's
     COLOUR is the resolving seat's (style:border-color above); this rule
     keeps its width/radius. */
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
  .resolved__meta {
    flex: 1;
    min-width: 0;
    padding-top: 0.2rem;
    display: flex;
    flex-direction: column;
    gap: 0.1rem;
  }
  .resolved__label {
    font-family: var(--font-data);
    font-size: var(--t-10);
    text-transform: uppercase;
    letter-spacing: 0.06em;
    color: var(--ink-faint);
  }
  .resolved__name {
    font-weight: 600;
    font-size: var(--t-10);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .resolved__art {
    flex: none;
    cursor: default;
  }
  .resolved__art:focus-visible {
    outline: 2px solid var(--initiative);
    outline-offset: 1px;
  }
</style>
