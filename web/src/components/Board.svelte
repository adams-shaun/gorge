<script lang="ts">
  import type { View, SeatInfo } from '../protocol';
  import type { CardOptions } from '../lib/cardoptions';
  import { quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import Quadrant from './Quadrant.svelte';
  import Arrows from './Arrows.svelte';

  /** Board lays out one Quadrant per player at quadrantFor(seat, seats, viewer) — a pure function of seat index and the VIEWER (so a 1v1 viewer lands at the bottom and their opponent at the top). The corner each seat landed in is passed on, because the quadrant draws its seat rule and its command area on the seat's outer edge and only the layout knows which edge that is. The stack goes with it for the command area alone: a commander mid-cast is a spell there rather than in any zone list. `options` is the pending decision's card-indexed offers (null for a spectator or when nothing is pending) — threaded straight to every tile so each card can carry its own options and be marked. */
  let { view, seats, options = null }: { view: View; seats: SeatInfo[]; options?: CardOptions | null } = $props();

  const CELL: Record<string, string> = {
    tl: 'top:0;left:0;width:50%;height:50%',
    tr: 'top:0;right:0;width:50%;height:50%',
    bl: 'bottom:0;left:0;width:50%;height:50%',
    br: 'bottom:0;right:0;width:50%;height:50%',
    l: 'top:0;left:0;width:50%;height:100%',
    r: 'top:0;right:0;width:50%;height:100%',
    // 1v1: the table is two full-width halves stacked, not two side-by-side
    // columns. The viewer (bottom) holds the lower half, the opponent (top)
    // the upper half.
    top: 'top:0;left:0;width:100%;height:50%',
    bottom: 'bottom:0;left:0;width:100%;height:50%',
  };
</script>

<div class="board">
  {#each view.players as p (p.seat)}
    {@const corner = quadrantFor(p.seat, view.players.length, view.viewer)}
    <div class="cell" style={`position:absolute;${CELL[corner]}`}>
      <Quadrant player={p} colour={seatColour(p.seat, seats)} {corner} stack={view.stack} {options} />
    </div>
  {/each}
  <Arrows {view} />
</div>

<style>
  .board {
    position: relative;
    width: 100%;
    height: 100%;
  }
  /* One pixel of the table's own ground between the seats. Without it four
     panels of the same colour read as one undivided field and the seat rules
     look like stray lines drawn across it. */
  .cell {
    padding: 1px;
  }
</style>
