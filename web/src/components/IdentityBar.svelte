<script lang="ts">
  import type { PlayerView, SeatInfo } from '../protocol';
  import type { SeatCorner } from '../lib/seattable';
  import type { CardOptions, TileOptions } from '../lib/cardoptions';
  import { playerOptions } from '../lib/cardoptions';
  import ManaPool from './ManaPool.svelte';
  import OptionPicker from './OptionPicker.svelte';

  /**
   * IdentityBar sits at one seat's outer corner and is the whole player
   * box, on exactly three lines (B1 / I-10: the user's spec — "player
   * name/state box is wasteful on real estate"):
   *
   *   1. the truncated player name, then the life-total bubble
   *   2. `library X  hand Y  graveyard Z` on one row (spacing is ours)
   *   3. the mana readout as bubbles, colourless included: what the seat
   *      could tap for right now (AVAILABLE, hollow chips — public, so it
   *      shows for every seat and fills this line in the ordinary case the
   *      floating pool cannot), plus the floating pool (solid chips) when
   *      any, visually distinct so the two are never mistaken for each other
   *
   * It used to also carry the deck name and the CR 903.10 commander-damage
   * clock. The user dropped the deck line because a deck is not a player
   * (B3, the name is now an independent PlayerName); the commander-damage
   * clock is gone because the three-line contract removed the clock from the
   * client entirely — a box may not be taller than three rows. The commander
   * tile (CommanderTile) on the board still tracks the command zone and is
   * out of scope here, but it renders no damage readout, so commander damage
   * is now unreadable anywhere in this client. Where that clock comes back is
   * an open product question, and this comment is deliberately not defending
   * a rationale for dropping it: it is a consequence of the spec, not a judged
   * removal.
   *
   * Seat colour stays a left rule and the active seat stays a full
   * perimeter in its own colour; LOST seats stay struck through and dimmed
   * (an eliminated seat's box reads greyed no matter what the life bubble
   * says, because a seat can lose with life untouched — commander damage,
   * an empty library, a concession).
   */
  let { player, seat, colour, active, priority, corner, options = null }: {
    player: PlayerView; seat?: SeatInfo; colour: string; active: boolean; priority: boolean;
    corner: SeatCorner;
    players?: PlayerView[];
    /** options is the same board-wide CardOptions index Quadrant hands its
     *  CardStacks (Table.svelte builds one from the active decision).
     *  IdentityBar looks this SEAT up in its byPlayer index (cardoptions.ts
     *  optionsByPlayer): a spell whose only legal target is a player (no
     *  creature on the board to target) previously marked nothing at all —
     *  optionsByObj has no way to represent "targets a player, not an
     *  object". This is the one place that gap closes. */
    options?: CardOptions | null;
  } = $props();

  const tileOptions = $derived<TileOptions | null>(options ? playerOptions(options, player.seat) : null);

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

  // A 1v1 seat's identity bar anchors to a corner of its own half, the
  // convention competitive clients use (Arena/MTGO: the opponent's box at the
  // top, yours at the bottom).
  //
  // The seated player's own box and hand are one bottom seat strip. The box
  // is flush to the board's bottom-left edge; HandFan reserves exactly its
  // --own-seat-w bay and starts alongside it. That makes the identity visibly
  // docked without putting it over the first card or floating it above both.
  const CORNER: Record<string, string> = {
    tl: 'top:var(--sp-2);left:var(--sp-2)', tr: 'top:var(--sp-2);right:var(--sp-2)',
    bl: 'bottom:var(--sp-2);left:var(--sp-2)', br: 'bottom:var(--sp-2);right:var(--sp-2)',
    l: 'top:var(--sp-2);left:var(--sp-2)', r: 'top:var(--sp-2);right:var(--sp-2)',
    top: 'top:var(--sp-2);right:var(--sp-2)',
    bottom: 'bottom:0;left:0;width:var(--own-seat-w, 12rem)',
  };
</script>

<div
  class="identity"
  class:active
  class:lost={player.lost}
  class:priority
  class:own={corner === 'bottom'}
  style={`position:absolute;${CORNER[corner]};--seat:${colour}`}
  data-seat={player.seat}
  data-tone={tileOptions?.tone ?? ''}
  data-options={tileOptions ? tileOptions.list.length : undefined}
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
    <ManaPool pool={player.pool} available={player.available} />
  </div>

  {#if tileOptions}
    <!-- The identity plate uses the same shared direct/radial/list control as
         card tiles; only its accessible subject differs. -->
    <OptionPicker {tileOptions} subject="targeting {who}" />
  {/if}
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
    /* Above the hand fan (6) and below the seat panel (8) and the hover
       CardDetail (9). The seated player's own box no longer overlaps the fan
       at all (see --own-hand-h above), so this ordering is a backstop for a
       narrow board rather than the thing keeping the two legible. */
    z-index: 7;
    backdrop-filter: blur(6px);
  }
  /* The active player is stated as a full perimeter in the seat's OWN colour
     — not a generic "it's someone's turn" hue — so the ring says both "the
     turn is here" and "here is who" in one glance. */
  /* Own identity is an edge plate, not a card floating over felt. Its right
     edge meets the hand's reserved bay at the same bottom baseline. */
  .identity.own {
    border-radius: 0 var(--radius) 0 0;
  }

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
