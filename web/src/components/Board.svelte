<script lang="ts">
  import type { View, SeatInfo } from '../protocol';
  import type { CardOptions } from '../lib/cardoptions';
  import { quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import Quadrant from './Quadrant.svelte';
  import Arrows from './Arrows.svelte';

  /** Board lays out one Quadrant per player at quadrantFor(seat, seats, viewer) — a pure function of seat index and the VIEWER (so a 1v1 viewer lands at the bottom and their opponent at the top). The corner each seat landed in is passed on, because the quadrant draws its seat rule and its command area on the seat's outer edge and only the layout knows which edge that is. The stack goes with it for the command area alone: a commander mid-cast is a spell there rather than in any zone list. `options` is the pending decision's card-indexed offers (null for a spectator or when nothing is pending) — threaded straight to every tile so each card can carry its own options and be marked. */
  let { view, seats, options = null, reserveCentre = false }: {
    view: View;
    seats: SeatInfo[];
    options?: CardOptions | null;
    /** Reserve the board clock's real centre lane instead of letting card rows continue beneath it. */
    reserveCentre?: boolean;
  } = $props();

  // A board with its clock mounted gives each half one edge of a real centre
  // lane. The clock may still be positioned over that empty lane, but cards
  // cannot enter it: the seat cells themselves end before the lane begins.
  // Without a clock (component fixtures and other embedders), --lane-half is
  // zero and these are byte-for-byte the old halves.
  const CELL: Record<string, string> = {
    tl: 'top:0;left:0;width:50%;height:calc(50% - var(--lane-half))',
    tr: 'top:0;right:0;width:50%;height:calc(50% - var(--lane-half))',
    bl: 'bottom:0;left:0;width:50%;height:calc(50% - var(--lane-half))',
    br: 'bottom:0;right:0;width:50%;height:calc(50% - var(--lane-half))',
    l: 'top:0;left:0;width:calc(50% - var(--lane-half));height:100%',
    r: 'top:0;right:0;width:calc(50% - var(--lane-half));height:100%',
    // 1v1: the table is two full-width halves stacked, not two side-by-side
    // columns. The viewer (bottom) holds the lower half, the opponent (top)
    // the upper half.
    top: 'top:0;left:0;width:100%;height:calc(50% - var(--lane-half))',
    bottom: 'bottom:0;left:0;width:100%;height:calc(50% - var(--lane-half))',
  };
</script>

<div class="board" class:centre-lane={reserveCentre}>
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
    --lane-half: 0px;
  }
  .board.centre-lane {
    /* Half the painted clock plus the named minimum clearance on this edge.
       This changes the actual seat boxes, rather than hoping an overlay happens
       not to cover their contents. */
    --lane-half: calc(var(--phase-track-row-h) / 2 + var(--phase-card-clearance));
  }
  /* One pixel of the table's own ground between the seats. Without it four
     panels of the same colour read as one undivided field and the seat rules
     look like stray lines drawn across it. */
  .cell {
    padding: 1px;
  }
</style>
