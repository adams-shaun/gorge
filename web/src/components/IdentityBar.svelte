<script lang="ts">
  import type { PlayerView, SeatInfo } from '../protocol';
  import { commanderDamageOf } from '../lib/commander';
  import ManaPool from './ManaPool.svelte';

  /**
   * IdentityBar sits at one seat's outer corner: who, life centred big, the
   * zone counts, an outline while active, a dot while holding priority,
   * ELIMINATED once lost. data-seat carries the seat index (not the seat
   * prop below, which is that seat's SeatInfo) for Task 22's arrows.
   *
   * Under life sits the second clock, commander damage (CR 903.10): one line
   * per commander that has actually hurt this seat, each with its own amount
   * and its own danger state — never a sum, because 11 from two different
   * commanders is not the 22 that is lethal. 21 from any single commander
   * is lethal regardless of life total, so the clock must read next to the
   * life it ignores; `players` is the match-wide roster commanderDamageOf
   * resolves dealer names from. The block renders nothing when the seat has
   * taken no commander damage.
   *
   * LOST SEATS. A seat can be eliminated with its life total untouched — 21+
   * commander damage from one commander, drawing from an empty library,
   * conceding — so the fix for "a dead player's box doesn't say so" is never
   * to force life to 0 (rules/sba.go's other loss conditions would make that
   * a lie). It is stating it in words: an ELIMINATED band, always drawn when
   * `player.lost`, that does not depend on or alter the life number beside
   * it. The elimination CAUSE (the PlayerLost event's Text) is the rail's
   * job, not this component's — this only says THAT the seat is out.
   *
   * FLOATING MANA. `.mana-row` is a fixed-height band, always rendered,
   * whether the seat has floating mana or not — see its own comment below
   * for why a reserved, non-grow-to-fit box is the actual point of the
   * feature. `player.pool` reaches ManaPool untouched, including when it is
   * a literal JSON `null` (every seat's pool on a public-spectator client,
   * view/view.go): ManaPool already treats a hidden pool as "draw nothing",
   * which is exactly the right reading here too — an unknown pool is not the
   * same claim as an empty one, and this component makes neither claim on
   * its own.
   */
  let { player, seat, colour, active, priority, corner, players = [] }: {
    player: PlayerView; seat?: SeatInfo; colour: string; active: boolean; priority: boolean;
    corner: 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r';
    players?: PlayerView[];
  } = $props();

  // The commander clock, resolved once per render: one entry per commander
  // that has dealt this seat damage (CR 903.10), never summed across
  // commanders — 11 + 11 is two entries, 21 from one is lethal.
  const cmdDamage = $derived(commanderDamageOf(player, players));

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
  {#if player.lost}<div class="eliminated" data-eliminated>Eliminated</div>{/if}
  {#if deck}<div class="deck">{deck}</div>{/if}
  <div class="life">{player.life}</div>
  <div class="mana-row" data-mana-row>
    <ManaPool pool={player.pool} />
  </div>
  {#if cmdDamage.length > 0}
    <div class="cmd" data-commander-damage>
      <span class="cmd-h">Commander</span>
      {#each cmdDamage as d (d.id)}
        <div
          class="cmd-row"
          class:crit={d.crit && !d.lethal}
          class:lethal={d.lethal}
          data-cmd-damage={d.id}
          data-cmd-amount={d.amount}
          data-cmd-state={d.lethal ? 'lethal' : d.crit ? 'critical' : 'low'}
          title="{d.amount} commander damage from {d.name} — {d.lethal ? 'lethal (CR 903.10)' : d.crit ? 'two or fewer from lethal' : ''}"
        >
          <span class="cmd-name">{d.name}</span>
          <span class="cmd-amount data">{d.amount}</span>
        </div>
      {/each}
    </div>
  {/if}
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
  /* The active player is stated as a full perimeter in the seat's OWN colour
     — not a generic "it's someone's turn" hue — so the ring says both "the
     turn is here" and "here is who" in one glance (survey report: two edges
     read as decoration, not as a signal you can find at a glance). The
     border goes fully saturated and thickens on all four sides at once
     (the shorthand below overrides the inactive state's left-only accent),
     and a soft matching glow pushes the whole plate visually forward without
     filling it with colour — a wash across the plate would compete with the
     life total and the card art it sits over. */
  .identity.active {
    border: 2px solid var(--seat);
    box-shadow:
      0 0 0 2px color-mix(in srgb, var(--seat) 45%, transparent),
      0 0 14px 2px color-mix(in srgb, var(--seat) 55%, transparent);
    background: color-mix(in srgb, var(--felt-raised) 92%, transparent);
  }
  .identity.lost .name,
  .identity.lost .life {
    text-decoration: line-through;
    color: var(--ink-faint);
  }
  /* Stated in words, not only implied by a struck-through life total that
     may not even have moved (commander damage, an empty library and a
     concession all end a seat without touching life). Danger red is already
     reserved for "this clock is done" (the lethal commander-damage row
     below), and this is exactly that state for the whole seat. */
  .eliminated {
    font-size: var(--t-10);
    font-weight: 700;
    letter-spacing: 0.05em;
    text-transform: uppercase;
    color: var(--danger);
    margin-top: 1px;
  }
  /* A fixed-height band for floating mana, ALWAYS rendered — this is the
     actual point of the feature (survey report: "player boxes resizing and
     bouncing around turn by turn"). ManaPool itself draws nothing at all for
     an empty or hidden pool (its own contract), so without a reserved slot
     around it the box would grow a row when mana is floated and shrink back
     the moment it is spent — exactly the bounce the report is about. The
     height here is fixed (not min-height, not padding sized to content), so
     the box is the identical height whether the pool is full, empty, or
     null (every seat but the viewer's own on a public-spectator client).
     overflow hidden is a backstop, never the normal case: six symbols at
     this size comfortably fit the box's own width before it would ever
     clip. */
  .mana-row {
    display: flex;
    align-items: center;
    justify-content: center;
    height: 1.2rem;
    margin: 0 0 var(--sp-2);
    overflow: hidden;
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
  /* The second clock. It shares life's corner because it overrides life
     completely: 21 from one commander is lethal whatever the number beside
     it says, so the two must be read as one pair, not cross-referenced
     across panels. Each commander is its own row with its own state. */
  .cmd {
    display: flex;
    flex-direction: column;
    gap: 1px;
    margin: calc(var(--sp-1) * -1) 0 var(--sp-2);
    text-align: left;
  }
  .cmd-h {
    font-size: var(--t-10);
    color: var(--ink-faint);
    letter-spacing: 0.02em;
  }
  .cmd-row {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: var(--sp-2);
    font-size: var(--t-12);
    line-height: 1.4;
    color: var(--ink-inst);
    border-left: 2px solid transparent;
    padding-left: 0.3em;
  }
  .cmd-name {
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .cmd-amount {
    font-family: var(--font-data);
    font-variant-numeric: tabular-nums;
    flex: none;
  }
  /* 19–20: two or fewer points from lethal; the seat rule is already the
     danger hue the client reserves for a clock that is nearly done. */
  .cmd-row.crit {
    color: var(--danger);
    font-weight: 600;
    border-left-color: var(--danger);
  }
  /* 21+: lethal. A game state that can only exist until the next state-based
     action, so it is drawn as already over. */
  .cmd-row.lethal {
    background: var(--danger);
    color: var(--mana-w);
    font-weight: 600;
    border-left-color: var(--felt-sunk);
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
