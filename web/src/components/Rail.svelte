<script lang="ts">
  import type { View, SeatInfo, DecisionBody } from '../protocol';
  import { seatColour } from '../lib/colours';
  import { visibleHand } from '../lib/board';
  import HandList from './HandList.svelte';
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
    <h3>Stack</h3>
    {#each topFirst as s, i (s.id)}
      <StackTile stack={s} {view} emphasized={emphasizeTop && i === 0} dimmed={emphasizeTop && i > 0} />
    {/each}
  </section>

  <section>
    <h3>Pending</h3>
    <PendingTray pending={view.pending} />
  </section>

  {#if decision}
    <p class="decision">Seat {decision.player} · {decision.kind} · {decision.prompt}</p>
  {/if}
</div>

<style>
  .rail-inner { display: flex; flex-direction: column; gap: 1rem; }
  h3 { margin: 0 0 .4rem; font-size: .8rem; opacity: .8; }
  .decision { font-size: .8rem; padding: .5rem; background: #1b1b1f; border-radius: 6px; }
</style>
