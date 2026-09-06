<script lang="ts">
  import type { View, SeatInfo } from '../protocol';
  import { quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import Quadrant from './Quadrant.svelte';
  import Arrows from './Arrows.svelte';

  /** Board lays out one Quadrant per player at quadrantFor(seat, seats), a pure function of seat index — so the layout is deterministic regardless of arrival order. The corner each seat landed in is passed on, because the quadrant draws its seat rule on the seat's outer edge and only the layout knows which edge that is. */
  let { view, seats }: { view: View; seats: SeatInfo[] } = $props();

  const CELL: Record<string, string> = {
    tl: 'top:0;left:0;width:50%;height:50%',
    tr: 'top:0;right:0;width:50%;height:50%',
    bl: 'bottom:0;left:0;width:50%;height:50%',
    br: 'bottom:0;right:0;width:50%;height:50%',
    l: 'top:0;left:0;width:50%;height:100%',
    r: 'top:0;right:0;width:50%;height:100%',
  };
</script>

<div class="board">
  {#each view.players as p (p.seat)}
    {@const corner = quadrantFor(p.seat, view.players.length)}
    <div class="cell" style={`position:absolute;${CELL[corner]}`}>
      <Quadrant player={p} colour={seatColour(p.seat, seats)} {corner} />
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
