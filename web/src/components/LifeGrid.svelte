<script lang="ts">
  import type { SeatInfo } from '../protocol';
  import { seatColour } from '../lib/colours';
  let { life, lost, seats = [], active = -1 }: { life: number[]; lost: boolean[]; seats?: SeatInfo[]; active?: number } = $props();
</script>

<div class="grid" style:--n={life.length}>
  {#each life as l, i (i)}
    <div class="seat" class:lost={lost[i]} class:active={i === active} style:background={seatColour(i, seats)} title={seats[i]?.name ?? `Seat ${i}`}>
      {l}
    </div>
  {/each}
</div>

<style>
  .grid { display: grid; grid-template-columns: repeat(2, 1fr); gap: 2px; }
  .seat { color: white; font: 700 1.6rem/1 system-ui, sans-serif; padding: .6rem .4rem; text-align: center; border-radius: 4px; opacity: .95; }
  .seat.lost { opacity: .35; text-decoration: line-through; }
  .seat.active { outline: 3px solid white; outline-offset: -3px; }
</style>
