<script lang="ts">
  import type { PlayerView, StackView } from '../protocol';
  import { attachedTo, groupBattlefield, stackIdentical } from '../lib/board';
  import CardStack from './CardStack.svelte';
  import CommandArea from './CommandArea.svelte';

  /** Quadrant shows one player's battlefield, split into the three rows board.ts groups it into, plus the seat's command area at its own rim (CommandArea, which draws nothing at all for a seat with no commander roster). It has no rules knowledge: grouping and ordering come entirely from groupBattlefield; stackIdentical then collapses interchangeable permanents within a row into one tile with a count (CardStack renders the group). Attachments still come from attachedTo for a group of one — a stacked group has none by the stacking rule. `stack` is passed through to the command area only: a commander mid-cast is a spell on the stack, not in any zone list. */
  let { player, colour, corner = 'bl', stack = [] }: { player: PlayerView; colour: string; corner?: 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r'; stack?: StackView[] } = $props();

  const battlefieldGroups = $derived(groupBattlefield(player.battlefield));
  const stacks = $derived({
    lands: stackIdentical(battlefieldGroups.lands),
    creatures: stackIdentical(battlefieldGroups.creatures),
    others: stackIdentical(battlefieldGroups.others),
  });

  // The seat rule goes on the seat's OUTER edge — the table's rim — so four
  // rules frame the table instead of four lines cutting across the middle of
  // it. quadrantFor already decided which corner this seat sits in.
  const OUTER: Record<string, string> = { tl: 'top', tr: 'top', bl: 'bottom', br: 'bottom', l: 'left', r: 'right' };
  // …and the same fact orients the board. Every seat's permanents are pushed
  // toward the middle of the table, lands at that seat's own rim and
  // creatures nearest the centre, the way four people actually sit round a
  // table. That is what puts the two creature rows of a combat next to each
  // other across the seam, and it leaves each seat's outer corner clear for
  // its identity bar instead of stranding a permanent underneath it.
  // Two seats share the full height side by side and both identity bars sit
  // along the TOP, so those two boards pack downward instead.
  const FACING: Record<string, string> = { tl: 'facing-down', tr: 'facing-down', bl: 'facing-up', br: 'facing-up', l: 'facing-side', r: 'facing-side' };
</script>

<div class="quadrant rule-{OUTER[corner]} {FACING[corner]}" style:--seat={colour}>
  <!-- Nonlands above, lands below (survey #22), so a board stays parseable as
       it grows and the row a combat is read from is always in the same place. -->
  <div class="row creatures">
    {#each stacks.creatures as g (g.key)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} />
    {/each}
  </div>
  <div class="row others">
    {#each stacks.others as g (g.key)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} />
    {/each}
  </div>
  <div class="row lands">
    {#each stacks.lands as g (g.key)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} />
    {/each}
  </div>
  <!-- The seat's own command zone, at the seat's own rim: last in the column
       so the auto margin below can park it against that rim, and outside the
       three rows so it can never displace one. Nothing is drawn for a seat
       with no roster. -->
  <CommandArea {player} {stack} {corner} />
</div>

<style>
  /*
   * Felt: the seat is identified by a rule on its outer edge, not by a
   * saturated wash over the whole quadrant. Four translucent colour fields
   * behind card art muddy every card on the table; a rule costs nothing and
   * says the same thing. Same vocabulary as the identity bar and life grid.
   */
  .quadrant {
    box-sizing: border-box;
    width: 100%;
    height: 100%;
    background: var(--felt-raised);
    padding: var(--sp-3);
    display: flex;
    flex-direction: column;
    gap: var(--sp-3);
    overflow: auto;
    overscroll-behavior: contain;
  }
  /* Seats along the top of the table read from their own rim inward, so the
     column runs the other way and packs against the seam. */
  .facing-down {
    flex-direction: column-reverse;
    justify-content: flex-start;
    padding-top: var(--sp-8);
  }
  .facing-up {
    flex-direction: column;
    justify-content: flex-start;
    padding-bottom: var(--sp-8);
  }
  .facing-side {
    flex-direction: column;
    justify-content: flex-end;
    padding-top: var(--sp-8);
  }

  /* The command area parks against the seat's own RIM, at the far end of the
     column from the seam the rows pack against. Which margin does that
     depends on which way this seat's column runs, so the rule lives here
     beside the flex-direction it depends on rather than in CommandArea:
     auto absorbs the whole gap between the last row and the area, so the
     area is at the rim whenever there is room and simply follows the rows
     into the quadrant's own scroll when there is not. Either way it takes no
     space from a row and moves none of them.

     facing-side packs its rows at the BOTTOM (justify-content: flex-end)
     while the seat's plate and free band are at the top, so there the area
     is ordered ahead of the rows and the auto margin pushes the rows away
     from it instead.

     The negative margin on the rim side gives the area the rim padding back.
     That padding keeps PERMANENTS off the corner the identity plate floats
     over; the command zone is the seat's own furniture, at the far end of
     that same band, and it belongs at the table's edge like the plate. It
     also buys back 32px: without it a four-seat commander quadrant measured
     254px of content in a 244px box at 1280x748 and scrolled with an empty
     board.

     Sticky at that same rim edge. A four-seat quadrant is 246px tall at
     1280x748 and one creature row is 145px, so a real Commander board
     already overflows and this box already scrolls; without the pin the
     command zone would scroll off the table exactly when the table is
     busiest, and the whole point of drawing it is that an opponent's
     commander can be read at any time. */
  .facing-up > :global(.command-area) {
    margin-top: auto;
    margin-bottom: calc(var(--sp-8) * -1);
    position: sticky;
    bottom: 0;
  }
  .facing-down > :global(.command-area) {
    margin-bottom: auto;
    margin-top: calc(var(--sp-8) * -1);
    position: sticky;
    top: 0;
  }
  .facing-side > :global(.command-area) {
    order: -1;
    margin-bottom: auto;
    margin-top: calc(var(--sp-8) * -1);
    position: sticky;
    top: 0;
  }

  .rule-top { border-top: 2px solid var(--seat); }
  .rule-bottom { border-bottom: 2px solid var(--seat); }
  .rule-left { border-left: 2px solid var(--seat); }
  .rule-right { border-right: 2px solid var(--seat); }

  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-3);
    align-items: flex-end;
    align-content: flex-end;
    min-height: var(--sp-2);
  }
  /* Creatures are drawn larger, because they are what combat reads. Lands and
     the rest are smaller: a board of eight Islands should not cost the same
     room as eight creatures. The two sizes are set here rather than by
     passing size="large" to the tile, because how big a permanent is drawn is
     a property of the row it sits in and not of the card.

     No row stretches. A creature row with flex:1 pushed the lands to the far
     edge and opened a hole in the middle of every seat that had two
     permanents; the rows simply sit together against the seam instead. */
  .row.creatures {
    align-items: flex-start;
    align-content: flex-start;
    --card-w: 104px;
  }
  .row.others,
  .row.lands {
    --card-w: 76px;
  }
</style>
