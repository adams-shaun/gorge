<script lang="ts">
  import type { PlayerView, SeatInfo } from '../protocol';
  import ManaPool from './ManaPool.svelte';

  /**
   * IdentityBar sits at one seat's outer corner and is the whole player
   * box, on exactly three lines (B1 / I-10: the user's spec — "player
   * name/state box is wasteful on real estate"):
   *
   *   1. the truncated player name, then the life-total bubble
   *   2. `library X  hand Y  graveyard Z` on one row (spacing is ours)
   *   3. the mana pool as bubbles, colourless included
   *
   * It used to also carry the deck name and the CR 903.10 commander-damage
   * clock, and the user dropped both: a deck is not a player (B3, the name
   * is now an independent PlayerName) and the commander tile — the real
   * command-zone control — lives on the board in the creatures row
   * (CommanderTile, out of scope here). Dropping the second clock is a
   * deliberate spec choice, not an accidental regression: the box is three
   * lines and that is the contract.
   *
   * Seat colour stays a left rule and the active seat stays a full
   * perimeter in its own colour; LOST seats stay struck through and dimmed
   * (an eliminated seat's box reads greyed no matter what the life bubble
   * says, because a seat can lose with life untouched — commander damage,
   * an empty library, a concession).
   */
  let { player, seat, colour, active, priority, corner }: {
    player: PlayerView; seat?: SeatInfo; colour: string; active: boolean; priority: boolean;
    corner: 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r';
    players?: PlayerView[];
  } = $props();

  // The table knows a seat's name; a bare host that never registered one does
  // not, and PlayerView always carries a name of its own. Falling straight
  // through to "Seat 2" while the rail two inches away calls the same player
  // something else is the kind of small incoherence that makes a product feel
  // unfinished, so the wire's name is preferred over the placeholder. The
  // deck is no longer shown here (B1), so the deck resolution is gone.
  const who = $derived(seat?.name ?? player.name ?? `Seat ${player.seat}`);

  // The player name is a DISPLAY concern for the 10-char limit (B3): the wire
  // always carries the full name, and only the box clips it. The full name
  // stays reachable in the title/aria-label.
  const DISPLAY_MAX = 10;
  const truncated = $derived(who.length > DISPLAY_MAX ? `${who.slice(0, DISPLAY_MAX)}…` : who);

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
  class:priority
  style={`position:absolute;${CORNER[corner]};--seat:${colour}`}
  data-seat={player.seat}
>
  <div class="name">
    <span
      class="dot"
      class:held={priority}
      title={priority ? 'has priority' : undefined}
      role={priority ? 'img' : undefined}
      aria-label={priority ? 'Has priority' : undefined}
      aria-hidden={priority ? undefined : 'true'}
    ></span>
    <span class="who" title={who} data-player-name={who}>{truncated}</span>
    <span class="life" title={`${player.life} life`} data-life>{player.life}</span>
  </div>
  <div class="counts">
    <span class="count">library {player.library_size}</span>
    <span class="count">hand {player.hand_size}</span>
    <span class="count">graveyard {player.graveyard_size}</span>
  </div>
  <div class="mana-row" data-mana-row>
    <ManaPool pool={player.pool} />
  </div>
</div>

<style>
  /*
   * Anchored to the seat's OUTER corner so it never collides with board
   * content (survey #25). The seat's colour is a left rule rather than a full
   * border: an outline in eight different hues around a four-seat table is
   * noise, a rule is identity.
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
  /* The active player is stated as a full perimeter in the seat's OWN colour
     — not a generic "it's someone's turn" hue — so the ring says both "the
     turn is here" and "here is who" in one glance. */
  .identity.active {
    border: 2px solid var(--seat);
    box-shadow:
      0 0 0 2px color-mix(in srgb, var(--seat) 45%, transparent),
      0 0 14px 2px color-mix(in srgb, var(--seat) 55%, transparent);
    background: color-mix(in srgb, var(--felt-raised) 92%, transparent);
  }
  .identity.lost .who,
  .identity.lost .life {
    text-decoration: line-through;
    color: var(--ink-faint);
  }
  /* Line 1: name then the life bubble, on one row. The name clips at the
     10-char display limit and keeps the full name in the title; the life is
     a bubble that sits beside it rather than a giant number that owns the
     box. */
  .name {
    display: flex;
    align-items: center;
    justify-content: center;
    gap: var(--sp-2);
    font-size: var(--t-14);
    font-weight: 600;
    line-height: 1.2;
  }
  .who {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .identity.priority .who {
    color: var(--initiative);
    text-decoration: underline 2px dotted;
    text-underline-offset: 0.16em;
  }
  .identity.lost.priority .who {
    color: var(--ink-faint);
    text-decoration: line-through;
  }
  /* The marker is always in the name line: only its paint changes. */
  .dot {
    width: 0.4em;
    height: 0.4em;
    border-radius: 999px;
    background: var(--initiative);
    flex: none;
    opacity: 0;
  }
  .dot.held {
    opacity: 1;
  }
  .life {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    font-size: var(--t-16);
    line-height: 1;
    color: var(--ink);
    background: color-mix(in srgb, var(--felt-raised) 55%, transparent);
    border: 1px solid var(--edge-felt);
    border-radius: 999px;
    padding: 0.15em 0.5em;
    flex: none;
  }
  /* Line 2: the three zone counts on one row, in the data face. */
  .counts {
    display: flex;
    justify-content: center;
    gap: var(--sp-3);
    margin: var(--sp-1) 0 var(--sp-2);
    font-size: var(--t-10);
    color: var(--ink-dim);
    white-space: nowrap;
  }
  .count {
    font-variant-numeric: tabular-nums;
  }
  /* Line 3: the floating mana pool as bubbles, always rendered so the box
     does not grow a row when mana is floated and shrink the moment it is
     spent (the bounce the old preallocated band existed to stop). */
  .mana-row {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 1.2rem;
    overflow: hidden;
  }
</style>
