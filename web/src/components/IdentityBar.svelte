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
  style={`position:absolute;${CORNER[corner]};--seat:${colour}`}
  data-seat={player.seat}
>
  <div class="name">
    {#if priority}<span class="dot" title="has priority"></span>{/if}
    {seat?.name ?? `Seat ${player.seat}`}
  </div>
  {#if seat?.deck}<div class="deck">{seat.deck}</div>{/if}
  <div class="life">{player.life}</div>
  <dl class="counts">
    <div><dt>Library</dt><dd>{player.library_size}</dd></div>
    <div><dt>Hand</dt><dd>{player.hand_size}</dd></div>
    <div><dt>Graveyard</dt><dd>{player.graveyard_size}</dd></div>
  </dl>
</div>

<style>
  /*
   * Anchored to the seat's OUTER corner so it never collides with board
   * content (survey #25), with life anchored in the bar's horizontal centre
   * like the Pro Tour shields. The seat's colour is a left rule rather than a
   * full border: an outline in eight different hues around a four-seat table
   * is noise, a rule is identity.
   */
  .identity {
    background: color-mix(in srgb, var(--felt-sunk) 88%, transparent);
    border: 1px solid var(--edge-felt);
    border-left: 3px solid var(--seat);
    border-radius: var(--radius);
    padding: var(--sp-2) var(--sp-3);
    min-width: 9rem;
    text-align: center;
    z-index: 5;
    backdrop-filter: blur(6px);
  }
  /* The active player is stated once, by the seat rule growing — not by a
     second colour competing with the first. */
  .identity.active {
    border-left-width: 6px;
    background: color-mix(in srgb, var(--felt-raised) 92%, transparent);
  }
  .identity.lost .name,
  .identity.lost .life {
    text-decoration: line-through;
    color: var(--ink-faint);
  }
  .name {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 0.35em;
    font-size: var(--t-14);
    font-weight: 600;
  }
  .dot {
    width: 0.45em;
    height: 0.45em;
    border-radius: 999px;
    background: var(--seat);
    flex: none;
  }
  .deck {
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  /* Life is the one place type is a visual element rather than a label. */
  .life {
    font-size: var(--t-28);
    font-weight: 600;
    line-height: 1.1;
    margin: var(--sp-1) 0;
    font-variant-numeric: tabular-nums;
  }
  .counts {
    display: flex;
    justify-content: center;
    gap: var(--sp-3);
    margin: 0;
    font-family: var(--font-data);
    font-size: 0.6875rem;
    color: var(--ink-faint);
  }
  .counts div {
    display: flex;
    gap: 0.3em;
  }
  .counts dt {
    font-weight: 400;
  }
  .counts dd {
    margin: 0;
    color: var(--ink-dim);
  }
</style>
