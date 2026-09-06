<script lang="ts">
  import type { SeatInfo } from '../protocol';
  import { seatColour } from '../lib/colours';
  let { life, lost, seats = [], active = -1 }: { life: number[]; lost: boolean[]; seats?: SeatInfo[]; active?: number } = $props();
</script>

<div class="grid">
  {#each life as l, i (i)}
    <div class="seat" class:lost={lost[i]} class:active={i === active} style:--seat={seatColour(i, seats)} title={seats[i]?.name ?? `Seat ${i}`}>
      {l}
    </div>
  {/each}
</div>

<style>
  /*
   * Seat colour is a rule, not a fill. Eight tables of four filled blocks is
   * thirty-two saturated rectangles competing for the eye, and the palette
   * rule is that saturation carries meaning rather than decoration. The rule
   * identifies the seat; the number is the information.
   */
  .grid {
    display: grid;
    grid-template-columns: repeat(2, 1fr);
    gap: 1px;
    background: var(--edge-felt);
    border: 1px solid var(--edge-felt);
  }
  .seat {
    background: var(--felt-sunk);
    border-left: 3px solid var(--seat);
    color: var(--ink);
    font-size: var(--t-20);
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    padding: var(--sp-3) var(--sp-2);
    text-align: center;
  }
  .seat.lost {
    color: var(--ink-faint);
    border-left-color: var(--ink-faint);
    text-decoration: line-through;
  }
  /* The active seat is stated the same way it is on the identity bar: the seat
     rule grows. One vocabulary, learned once. */
  .seat.active {
    border-left-width: 7px;
    background: var(--felt-raised);
  }
</style>
