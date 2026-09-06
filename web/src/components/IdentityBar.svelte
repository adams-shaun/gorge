<script lang="ts">
  import type { PlayerView, SeatInfo } from '../protocol';

  /**
   * IdentityBar sits at one seat's outer corner: who, life centred big, the
   * zone counts, an outline while active, a dot while holding priority,
   * strike-through once lost. data-seat carries the seat index (not the seat
   * prop below, which is that seat's SeatInfo) for Task 22's arrows.
   */
  let { player, seat, colour, active, priority, corner }: {
    player: PlayerView; seat?: SeatInfo; colour: string; active: boolean; priority: boolean;
    corner: 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r';
  } = $props();

  // The table knows a seat's name; a bare host that never registered one does
  // not, and PlayerView always carries a name of its own. Falling straight
  // through to "Seat 2" while the rail two inches away calls the same player
  // "dimir-tempo" is the kind of small incoherence that makes a product feel
  // unfinished, so the wire's name is preferred over the placeholder.
  const who = $derived(seat?.name ?? player.name ?? `Seat ${player.seat}`);
  // …and the deck line is dropped when it would only repeat that name.
  const deck = $derived(seat?.deck && seat.deck !== who ? seat.deck : null);

  const CORNER: Record<string, string> = {
    tl: 'top:var(--sp-2);left:var(--sp-2)', tr: 'top:var(--sp-2);right:var(--sp-2)',
    bl: 'bottom:var(--sp-2);left:var(--sp-2)', br: 'bottom:var(--sp-2);right:var(--sp-2)',
    l: 'top:var(--sp-2);left:var(--sp-2)', r: 'top:var(--sp-2);right:var(--sp-2)',
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
    {who}
  </div>
  {#if deck}<div class="deck">{deck}</div>{/if}
  <div class="life">{player.life}</div>
  <dl class="counts">
    <div><dt>Library</dt><dd class="data">{player.library_size}</dd></div>
    <div><dt>Hand</dt><dd class="data">{player.hand_size}</dd></div>
    <div><dt>Graveyard</dt><dd class="data">{player.graveyard_size}</dd></div>
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
    gap: 0.4em;
    font-size: var(--t-14);
    font-weight: 600;
    line-height: 1.2;
  }
  /* Priority is the initiative, and the initiative has its own colour in this
     palette. Repeating the seat hue here would say "seat" twice and "whose
     turn it is to act" not at all. */
  .dot {
    width: 0.4em;
    height: 0.4em;
    border-radius: 999px;
    background: var(--initiative);
    flex: none;
  }
  .deck {
    font-size: var(--t-11);
    color: var(--ink-dim);
    line-height: 1.3;
  }
  /* Life is the one place type is a visual element rather than a label. */
  .life {
    font-size: var(--t-28);
    font-weight: 600;
    line-height: 1.05;
    margin: var(--sp-1) 0 var(--sp-2);
    font-variant-numeric: tabular-nums;
  }
  /* Labels are words and read in the interface face; the counts beside them
     are values and read in the data face. Mono is for values, never for
     small labels — the labels had been set in mono, which is the tell of a
     dashboard rather than an instrument. */
  .counts {
    display: flex;
    justify-content: center;
    gap: var(--sp-3);
    margin: 0;
    font-size: var(--t-10);
  }
  .counts div {
    display: flex;
    align-items: baseline;
    gap: 0.35em;
  }
  .counts dt {
    color: var(--ink-faint);
    font-weight: 400;
  }
  .counts dd {
    margin: 0;
    font-size: var(--t-10);
    color: var(--ink-dim);
  }
</style>
