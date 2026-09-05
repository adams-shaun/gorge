<script lang="ts">
  import type { View, SeatInfo } from '../protocol';
  import { quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import Quadrant from './Quadrant.svelte';
  import Arrows from './Arrows.svelte';

  /** Board lays out one Quadrant per player at quadrantFor(seat, seats), a pure function of seat index — so the layout is deterministic regardless of arrival order. */
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
    <div class="cell" style={`position:absolute;${CELL[quadrantFor(p.seat, view.players.length)]}`}>
      <Quadrant player={p} colour={seatColour(p.seat, seats)} />
    </div>
  {/each}
  <Arrows {view} />
</div>

<style>
  .board { position: relative; width: 100%; height: 100%; }
</style>
