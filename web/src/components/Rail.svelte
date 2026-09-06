<script lang="ts">
  import type { View, SeatInfo, DecisionBody } from '../protocol';
  import { seatColour } from '../lib/colours';
  import { visibleHand } from '../lib/board';
  import HandList from './HandList.svelte';
  import ZoneViewer from './ZoneViewer.svelte';
  import StackTile from './StackTile.svelte';
  import PendingTray from './PendingTray.svelte';

  /** Rail is every hand, the stack, the pending tray, and the live decision line — in that order, per the survey. emphasizeTop (seat view) applies survey item 10 — top-of-stack by contrast — while the spectator path leaves it off, unchanged. */
  let { view, seats, decision, emphasizeTop = false }: { view: View; seats: SeatInfo[]; decision: DecisionBody | null; emphasizeTop?: boolean } = $props();

  // view.stack lists bottom of the stack first (push order); the rail shows
  // what resolves next at the top, so it is reversed for display only.
  const topFirst = $derived([...view.stack].reverse());
</script>

<div class="rail-inner">
  {#each view.players as p (p.seat)}
    {#if visibleHand(p) !== null}
      <HandList player={p} deck={seats[p.seat]?.deck} colour={seatColour(p.seat, seats)} />
    {/if}
  {/each}

  <section>
    <h3>Zones</h3>
    {#each view.players as p (p.seat)}
      <ZoneViewer player={p} colour={seatColour(p.seat, seats)} />
    {/each}
  </section>

  <section>
    <h3>Stack{#if topFirst.length > 0} <span class="count">{topFirst.length}</span>{/if}</h3>
    {#each topFirst as s, i (s.id)}
      <StackTile stack={s} {view} emphasized={emphasizeTop && i === 0} dimmed={emphasizeTop && i > 0} />
    {/each}
  </section>

  <section>
    <h3>Pending</h3>
    <PendingTray pending={view.pending} />
  </section>

  {#if decision}
    <p class="decision">
      <span class="who">Seat {decision.player}</span>
      {decision.prompt}
    </p>
  {/if}
</div>

<style>
  /*
   * The instrument register: cooler and flatter than the felt, structured by
   * hairlines rather than by cards. Sections are divided, not boxed — a
   * stack of identically-rounded panels would read as chrome, and this rail
   * is meant to read as an instrument face.
   */
  .rail-inner {
    display: flex;
    flex-direction: column;
  }
  .rail-inner > :global(section),
  .rail-inner > :global(*) {
    padding: var(--sp-3);
    border-bottom: 1px solid var(--edge-inst);
  }
  .rail-inner > :global(*:last-child) {
    border-bottom: 0;
  }
  h3 {
    margin: 0 0 var(--sp-2);
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
