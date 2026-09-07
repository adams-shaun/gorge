<script lang="ts">
  import type { SeatInfo } from '../protocol';
  import { seatColour } from '../lib/colours';
  // `names` prints each seat's deck beside its life total. It is off by
  // default: in a compact overview cell the name does not fit and the title
  // attribute is the affordance. A roomy cell (few tables on the page) can
  // carry it, and then the cell answers "what game is this?" as well as
  // "how is it going?".
  let { life, lost, seats = [], active = -1, names = false }: { life: number[]; lost: boolean[]; seats?: SeatInfo[]; active?: number; names?: boolean } = $props();
</script>

<div class="grid" class:named={names}>
  {#each life as l, i (i)}
    <div class="seat" class:lost={lost[i]} class:active={i === active} style:--seat={seatColour(i, seats)} title={seats[i]?.name ?? `Seat ${i}`}>
      <!-- Only a real name is printed. "Seat 2" in 11px above a 40px life
           total is a placeholder pretending to be information, and the seat
           rule already says which seat this is. Names arrive with
           `match_start`, so a viewer who joins mid-match sees the totals
           alone until the next one begins. -->
      {#if names && seats[i]?.name}<span class="who">{seats[i].name}</span>{/if}
      <span class="life">{l}</span>
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
    /* Rows share the height the grid is given, so the same component is a
       chip in a dense lobby and a wallboard tile in a roomy one. */
    grid-auto-rows: 1fr;
    gap: 1px;
    background: var(--edge-felt);
    border: 1px solid var(--edge-felt);
  }
  .seat {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: var(--sp-1);
    background: var(--felt-sunk);
    border-left: 3px solid var(--seat);
    color: var(--ink);
    /* The life total's size is the container's call: --life-size is set by
       whoever knows how much room the grid was given. */
    font-size: var(--life-size, var(--t-20));
    font-weight: 600;
    font-variant-numeric: tabular-nums;
    line-height: 1;
    padding: var(--sp-3) var(--sp-2);
    text-align: center;
    min-width: 0;
  }
  /* The deck name is a label, not a value: instrument-small, dim, and it
     truncates rather than wrapping the tile out of shape. */
  .who {
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    font-size: var(--t-11);
    font-weight: 400;
    color: var(--ink-dim);
  }
  .seat.lost .who {
    color: var(--ink-faint);
  }
  /* The strike lands on the life total alone: a decoration on the tile would
     propagate into the deck name, which is not the thing that died. */
  .seat.lost .life {
    text-decoration: line-through;
  }
  .seat.lost {
    color: var(--ink-faint);
    border-left-color: var(--ink-faint);
  }
  /* The active seat is stated the same way it is on the identity bar: the seat
     rule grows. One vocabulary, learned once. */
  .seat.active {
    border-left-width: 7px;
    background: var(--felt-raised);
  }
</style>
