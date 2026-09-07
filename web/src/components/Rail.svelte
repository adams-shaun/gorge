<script lang="ts">
  import type { View, SeatInfo, DecisionBody } from '../protocol';
  import { seatColour } from '../lib/colours';
  import { visibleHand } from '../lib/board';
  import { focusSeat } from '../lib/seattable';
  import SeatTable from './SeatTable.svelte';
  import HandList from './HandList.svelte';
  import ZoneViewer from './ZoneViewer.svelte';
  import ManaPool from './ManaPool.svelte';
  import StackTile from './StackTile.svelte';
  import PendingTray from './PendingTray.svelte';

  /**
   * The rail is ONE table across seats, then the stack, the pending tray and
   * the live decision line (U4, and the survey's rail order).
   *
   * It used to be a panel per seat — a hand list, a command zone and a zone
   * strip each — which measured 1732px of content in a 496px slot on a
   * four-seat omniscient Commander table: everything from the third seat
   * down, the command zone included, was below the fold on the only format
   * the command zone exists for (U2). The fix is one layout pass, not two:
   *
   *   - Facts every seat has (life, hand, library, graveyard) become COLUMNS
   *     of one table, so they are compared down a column instead of hunted
   *     across four panels — and four seats cost four rows, not four panels.
   *   - The command zone LEFT this rail entirely (CZ1). A line of text is
   *     not how anyone recognises a commander, so each seat's commanders are
   *     drawn as art tiles in a command area at that seat's own rim on the
   *     board, with the CR 903.8 tax on the tile. Nothing takes the section's
   *     place here: the rail is one section shorter.
   *   - The genuinely per-seat LISTS — the hand, the graveyard and exile
   *     cards, the floating mana — are shown for ONE seat at a time in the
   *     detail pane, which follows the active player until the reader picks
   *     a row. Four hands stacked is precisely what did not fit.
   *
   * The rail then never scrolls as a whole: every section is intrinsically
   * sized or explicitly capped except the STACK, which takes the leftover
   * height and scrolls inside itself. Nothing can push a section off the
   * bottom, because nothing but the stack grows.
   *
   * Revisited (U-rail-2, "the stack is very important in a lot of games, we
   * can hardly see 1 card"): the stack used to be capped small like every
   * other section and the detail pane (hand/pool/zones) was the one that
   * grew, which is backwards — the stack is read constantly during a
   * spectated game and the detail pane is read occasionally, on demand. So
   * the flex roles swap: the detail pane becomes a capped, collapsible
   * section like pending always was, and the stack becomes the ONE region
   * with `flex: 1` — it fills whatever height the seat table, the detail
   * pane and pending do not need, at any viewport, because it is still
   * capped by a `min-height` rather than a fixed one (a fixed height is
   * exactly the mistake this rail was rewritten to stop making: it is either
   * too small on a tall screen or overflows a short one).
   *
   * emphasizeTop (seat view) applies survey item 10 — top-of-stack by
   * contrast — while the spectator path leaves it off, unchanged.
   */
  let {
    view,
    seats,
    decision,
    emphasizeTop = false,
    events = [],
  }: {
    view: View;
    seats: SeatInfo[];
    decision: DecisionBody | null;
    emphasizeTop?: boolean;
    /** the DVR's own event list, forwarded to SeatTable for the PlayerLost cause (Task: dead seats say why) and read by no one else here. Optional so every existing caller/test keeps rendering exactly as before with no cause shown. */
    events?: { event: { kind: string; player: number; text?: string } }[];
  } = $props();

  // The reader's explicit pick, or null to follow (focusSeat decides what
  // "follow" means). Pressing the row that is already focused releases the
  // pick and resumes following, so there is no separate "unpin" control.
  let picked = $state<number | null>(null);
  const focus = $derived(focusSeat(picked, view));
  const focused = $derived(view.players.find((p) => p.seat === focus) ?? null);

  // view.stack lists bottom of the stack first (push order); the rail shows
  // what resolves next at the top, so it is reversed for display only.
  const topFirst = $derived([...view.stack].reverse());
</script>

<div class="rail-inner">
  <SeatTable {view} {seats} {focus} {events} onFocus={(s) => (picked = picked === s ? null : s)} />

  <section class="focus" data-focus-pane data-focus-seat={focused?.seat}>
    {#if focused}
      <!-- Keyed on the seat so switching rows remounts the pane: the hover
           panel's HoverCard, and ZoneViewer's per-zone disclosure, belong to
           the seat being read and must not carry over to the next one. -->
      {#key focused.seat}
        {#if visibleHand(focused) !== null}
          <HandList player={focused} deck={seats[focused.seat]?.deck} colour={seatColour(focused.seat, seats)} />
        {:else}
          <p class="redacted" data-hand-redacted>
            {focused.name}'s hand is not visible — {focused.hand_size}
            {focused.hand_size === 1 ? 'card' : 'cards'}
          </p>
        {/if}
        <ManaPool pool={focused.pool} />
        <ZoneViewer player={focused} colour={seatColour(focused.seat, seats)} />
      {/key}
    {/if}
  </section>

  {#if decision}
    <p class="decision">
      <span class="who">Seat {decision.player}</span>
      {decision.prompt}
    </p>
  {/if}

  <!-- The pending tray sits directly above the transcript and directly below
       the stack, because it is literally what is about to become stack
       (design system, "Layout"); the transcript is the band under this rail. -->
  <section class="stack">
    <h3>Stack{#if topFirst.length > 0} <span class="count">{topFirst.length}</span>{/if}</h3>
    {#each topFirst as s, i (s.id)}
      <StackTile stack={s} {view} emphasized={emphasizeTop && i === 0} dimmed={emphasizeTop && i > 0} />
    {/each}
  </section>

  <section class="pending">
    <h3>Pending</h3>
    <PendingTray pending={view.pending} />
  </section>
</div>

<style>
  /*
   * The instrument register: cooler and flatter than the felt, structured by
   * hairlines rather than by cards. Sections are divided, not boxed — a
   * stack of identically-rounded panels would read as chrome, and this rail
   * is meant to read as an instrument face.
   *
   * The column is the height contract. Everything is capped or intrinsically
   * sized except the STACK, which is the one section with `flex: 1` and
   * takes exactly the leftover; the detail pane and the pending tray are
   * capped and scroll inside themselves instead. So the rail's content is
   * the rail's height whatever the game does, and no section can push
   * another off the bottom (U2) — and the section that most needs the room
   * (the stack, U-rail-2) is the one that gets it.
   */
  .rail-inner {
    display: flex;
    flex-direction: column;
    height: 100%;
    min-height: 0;
  }
  .rail-inner > :global(section),
  .rail-inner > :global(*) {
    flex: none;
    padding: var(--sp-2) var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
  }
  .rail-inner > :global(*:last-child) {
    border-bottom: 0;
  }
  /* Written as element+class deliberately: the rule above is
     `.rail-inner > section`, which carries an element's worth of specificity,
     so a bare `.focus` loses to it and the whole column silently reverts to
     flex: none — which is exactly how the first cut of this still overflowed
     by 250px. */
  section.focus {
    flex: 0 1 auto;
    min-height: 2.25rem;
    max-height: 11rem;
    overflow-y: auto;
  }
  /* The stack is the rail's primary region (U-rail-2): the only section that
     grows, so it fills whatever the seat table, the detail pane and pending
     leave over — at a tall viewport that is many entries at once, at a short
     one it is still more than the fixed cap this replaced ever gave it, and
     `min-height` (not a fixed height) is what keeps it from ever demanding
     more room than a short viewport has. */
  section.stack {
    flex: 1 1 8rem;
    min-height: 6rem;
    overflow-y: auto;
  }
  section.pending {
    flex: 0 1 auto;
    min-height: 2.25rem;
    max-height: 5rem;
    overflow-y: auto;
  }
  h3 {
    margin: 0 0 var(--sp-1);
    font-size: var(--t-12);
    font-weight: 600;
    color: var(--ink-dim);
    display: flex;
    align-items: baseline;
    gap: 0.4em;
  }
  /* The depth is always visible, panel open or closed (survey #11). */
  .count {
    font-family: var(--font-data);
    font-size: 0.6875rem;
    color: var(--ink-faint);
  }
  /* A hand this viewer may not see is a STATE, said in words. It is not an
     empty list, which would read as "they have no cards". */
  .redacted {
    margin: 0;
    font-size: var(--t-12);
    line-height: 1.4;
    color: var(--ink-faint);
  }
  .decision {
    margin: 0;
    font-size: var(--t-12);
    line-height: 1.4;
    color: var(--ink-inst);
  }
  .who {
    display: block;
    font-family: var(--font-data);
    font-size: 0.6875rem;
    color: var(--ink-faint);
  }
</style>
