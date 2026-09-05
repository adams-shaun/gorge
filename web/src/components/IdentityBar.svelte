<script lang="ts">
  import type { PlayerView, SeatInfo } from '../protocol';

  /**
   * IdentityBar sits at one seat's outer corner: name/deck, life centred big,
   * the zone counts, an outline while active, a dot while holding priority,
   * strike-through once lost. data-seat carries the seat index (not the seat
   * prop below, which is that seat's SeatInfo) for Task 22's arrows.
   */
  let { player, seat, colour, active, priority, corner }: {
    player: PlayerView; seat?: SeatInfo; colour: string; active: boolean; priority: boolean;
    corner: 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r';
  } = $props();

  const CORNER: Record<string, string> = {
    tl: 'top:.5rem;left:.5rem', tr: 'top:.5rem;right:.5rem',
    bl: 'bottom:.5rem;left:.5rem', br: 'bottom:.5rem;right:.5rem',
    l: 'top:.5rem;left:.5rem', r: 'top:.5rem;right:.5rem',
  };
</script>

<div
  class="identity"
  class:active
  class:lost={player.lost}
  style={`position:absolute;${CORNER[corner]};border-color:${colour}`}
  data-seat={player.seat}
>
  <div class="name">
    {seat?.name ?? `Seat ${player.seat}`}
    {#if priority}<span class="dot" style:background={colour} title="has priority"></span>{/if}
  </div>
  {#if seat?.deck}<div class="deck">{seat.deck}</div>{/if}
  <div class="life">{player.life}</div>
  <div class="counts">{player.library_size} lib · {player.hand_size} hand · {player.graveyard_size} gy</div>
</div>

<style>
  .identity {
    background: #1b1b1fdd; border: 2px solid transparent; border-radius: 8px;
    padding: .4rem .7rem; min-width: 8rem; text-align: center; z-index: 5;
  }
  .identity.active { box-shadow: 0 0 0 2px currentColor inset; }
  .identity.lost .name, .identity.lost .life { text-decoration: line-through; opacity: .5; }
  .name { font-weight: 600; display: flex; align-items: center; justify-content: center; gap: .3rem; }
  .dot { width: .5em; height: .5em; border-radius: 999px; display: inline-block; }
  .deck { font-size: .7rem; opacity: .7; }
  .life { font: 700 1.6rem/1 system-ui, sans-serif; margin: .2rem 0; }
  .counts { font-size: .65rem; opacity: .7; }
</style>
