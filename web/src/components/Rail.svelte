<script lang="ts">
  import type { View, SeatInfo, DecisionBody } from '../protocol';
  import { focusSeat } from '../lib/seattable';
  import SeatTable from './SeatTable.svelte';
  import ManaPool from './ManaPool.svelte';
  import StackTile from './StackTile.svelte';
  import PendingTray from './PendingTray.svelte';

  /**
   * The rail starts with ONE compact summary across seats, then the stack,
   * pending tray and live decision line. The summary supersedes the old split
   * between SeatTable counts and a second ZoneViewer count panel: life, hand,
   * library, graveyard and exile now appear exactly once. Disclosable pile
   * lists open in a body-portalled modal, so they cannot consume rail height.
   * The command zone remains on the board as art tiles (CZ1).
   *
   * The rail never scrolls as a whole: every section is intrinsically sized
   * or explicitly capped except the STACK, which takes the leftover height
   * and scrolls inside itself.
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
    showLog = true,
    onToggleLog = null,
  }: {
    view: View;
    seats: SeatInfo[];
    decision: DecisionBody | null;
    emphasizeTop?: boolean;
    /** the DVR's own event list, forwarded to SeatTable for the PlayerLost cause (Task: dead seats say why) and read by no one else here. Optional so every existing caller/test keeps rendering exactly as before with no cause shown. */
    events?: { event: { kind: string; player: number; text?: string } }[];
    /** showLog is whether the transcript is shown right now; the toggle below
     *  flips it. It is owned by Table.svelte (persisted per table and seat/
     *  spectator scope, the stops contract) and merely surfaced here next to
     *  the other seat controls. Optional so every existing caller/test renders
     *  as before with the control in its default state. */
    showLog?: boolean;
    /** onToggleLog is the rail's control: a real button (keyboard reachable,
     *  role switch) that asks Table to flip the transcript's visibility. */
    onToggleLog?: (() => void) | null;
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

  // The live decision line names the seat being asked. `seats` is the first
  // word (the table's registered name), but on the live table route it can be
  // EMPTY while the view still carries each player's own name (the one-shot
  // seed in Table.svelte races the async tables.lookup — see the ui15
  // report); the fallback must be that name, not the 0-based `Seat N`
  // placeholder, exactly as IdentityBar does.
  const decisionWho = $derived(
    decision === null
      ? ''
      : seats[decision.player]?.name ?? view.players.find((p) => p.seat === decision.player)?.name ?? `Seat ${decision.player}`,
  );
</script>

<div class="rail-inner">
  <div class="logbar">
    <span class="logbar__label">Log</span>
    {#if onToggleLog}
      <button
        class="logbar__toggle"
        class:on={showLog}
        type="button"
        role="switch"
        aria-checked={showLog}
        aria-label="Show the game log"
        data-log-toggle
        onclick={() => onToggleLog()}
      >
        <span class="dot" aria-hidden="true"></span>
        <span class="word">{showLog ? 'Visible' : 'Hidden'}</span>
      </button>
    {/if}
  </div>
  <SeatTable {view} {seats} {focus} {events} onFocus={(s) => (picked = picked === s ? null : s)} />

  <section class="focus" data-focus-pane data-focus-seat={focused?.seat}>
    {#if focused}
      <ManaPool pool={focused.pool} />
    {/if}
  </section>

  {#if decision}
    <p class="decision">
      <span class="who">{decisionWho}</span>
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
  /* The log visibility control sits at the rail's top, beside the seat
     controls. It is a dotted switch like the auto/skip switches in the seat
     panel, not a labelled button: its own state is the message, and the
     instrument never spells out an enum it can just show. */
  .logbar {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: var(--sp-2);
    padding: var(--sp-2) var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
    flex: none;
  }
  .logbar__label {
    font-size: 0.6875rem;
    letter-spacing: 0.03em;
    text-transform: uppercase;
    color: var(--ink-faint);
  }
  .logbar__toggle {
    display: inline-flex;
    align-items: center;
    gap: 0.45em;
    background: var(--instrument-raised);
    color: var(--ink-dim);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius);
    padding: 0.15rem var(--sp-2);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    font-weight: 600;
    cursor: pointer;
    flex: none;
  }
  .logbar__toggle.on {
    color: var(--felt-sunk);
    background: var(--offered);
    border-color: var(--offered);
  }
  .logbar__toggle .dot {
    width: 6px;
    height: 6px;
    border-radius: 50%;
    background: currentColor;
    opacity: 0.5;
  }
  .logbar__toggle.on .dot {
    opacity: 1;
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
