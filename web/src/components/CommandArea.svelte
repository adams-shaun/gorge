<script lang="ts">
  import type { PlayerView, StackView } from '../protocol';
  import { commandZoneOf, stackIdsOf } from '../lib/commander';
  import CommanderTile from './CommanderTile.svelte';

  /**
   * The command zone, on the table, where a command zone actually is: a small
   * area at one seat's own rim, holding one card-sized tile per roster
   * commander in genesis order (CR 903.6). It replaces the rail's Commanders
   * section entirely — the user asked for one home for commanders and chose
   * the board, so the CR 903.8 tax the rail carried is on the tile now.
   *
   * WHERE IT SITS, and why it is not literally in the corner. The quadrant
   * keeps its seat's outer rim clear of permanents — every seat's cards are
   * pushed toward the middle of the table — and IdentityBar is already
   * absolutely positioned in the outermost corner of that band, over the
   * quadrant, at a width that grows with the seat's name and deck (measured
   * 188–241px). So the area takes the same rim band, at the OPPOSITE
   * horizontal end from the identity plate: the two are the seat's own
   * furniture at the two ends of its own rim, above its own seat rule, and
   * they cannot collide however long a deck name gets — which a fixed gutter
   * beside the plate could not promise. It then stands off the seam by
   * --seam-clear, which keeps it clear of RecentStrip (132px wide, centred on
   * the board's bottom edge, so 66px of it reaches into each bottom seat's
   * inner corner) and puts a seat's tiles nearer its own plate than its
   * neighbour's. `corner` is the prop Board already passes Quadrant; nothing
   * here re-derives which corner a seat is in.
   *
   * WHY IT NEVER DISPLACES A ROW. The area is the last flex child of the
   * quadrant's column and takes the free space at the rim end with an auto
   * margin, so it is pushed as far from the three rows as the quadrant
   * allows. The rows pack from the SEAM end in every facing, so nothing here
   * can move them: with the board full the quadrant scrolls at the rim end,
   * exactly as it already did, and the rows stay where they were. Measured
   * before and after at 1280x748 and 1440x900 — identical row geometry.
   *
   * It is sticky at that rim edge, because a four-seat Commander quadrant is
   * 246px tall at 1280x748 and a single creature row is 145px: a real board
   * overflows and scrolls, and an area that scrolled away with it would take
   * the opponent's commander off the table exactly when the table is busy.
   * Pinned, it draws over the rows that pass beneath it — which is why it has
   * a recessed ground of its own: the command zone is a marked-out area of
   * the table, not four more permanents.
   *
   * A CONSTRUCTED TABLE DRAWS NOTHING. Not an empty frame, not a heading, not
   * a slot: no roster means no element at all. The rail's old panel left a
   * hole in exactly the format the command zone does not apply to (U2/U24)
   * and it must not come back on the felt.
   */
  let { player, stack = [], corner = 'bl' }: {
    player: PlayerView;
    stack?: StackView[];
    corner?: 'tl' | 'tr' | 'bl' | 'br' | 'l' | 'r';
  } = $props();

  const commanders = $derived(commandZoneOf(player, stackIdsOf(stack)));

  // The identity plate is at the left of the rim for tl/bl/l and at the right
  // for tr/br/r (IdentityBar's CORNER map); the area takes the other end.
  const SIDE: Record<string, string> = { tl: 'end', bl: 'end', l: 'end', tr: 'start', br: 'start', r: 'start' };
</script>

{#if commanders.length > 0}
  <div class="command-area side-{SIDE[corner]}" data-command-area data-seat={player.seat}>
    {#each commanders as c (c.commander.id)}
      <CommanderTile status={c} player={player.name} />
    {/each}
  </div>
{/if}

<style>
  /*
   * A row of card-sized tiles, smaller than the lands row: the command zone is
   * a corner of the table, not a fourth battlefield row, and it must read as
   * one glance of "who is this player" rather than as more permanents. The
   * ground is one step BELOW the felt, the way a playmat's command zone is
   * printed into the mat — a recess, not a raised panel, so it never competes
   * with the cards lying on it and so the tiles are still separable from the
   * battlefield when the area is pinned over it.
   *
   * The area is sized to its tiles rather than to the quadrant (align-self
   * rather than justify-content), so the recess is the zone and not a band
   * across the whole seat.
   *
   * margin-block-start:auto is what parks it at the rim. The quadrant's
   * column runs `column` (a seat at the bottom of the table), `column-reverse`
   * (a seat at the top) or `column` + justify-content:flex-end (a seat down
   * the side); in each case the rows pack against the SEAM and the free space
   * is at the rim, so the auto margin on the area's seam-facing side takes
   * all of it and the area ends up against the seat's own edge. That half and
   * the sticky pinning live in Quadrant.svelte, next to the flex-direction
   * rules they depend on.
   */
  .command-area {
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-2);
    /* Horizontal padding only: the recess frames the tiles, and every
       vertical pixel here is one the rows do not get in a 246px quadrant. */
    padding: 0 var(--sp-2);
    border-radius: var(--radius);
    background: color-mix(in srgb, var(--felt-sunk) 82%, transparent);
    box-shadow: inset 0 0 0 1px color-mix(in srgb, var(--edge-felt) 70%, transparent);
    /* Between the lands row (76px) and the creatures row (104px): a commander
       is not another permanent, but at 72px the blank's clipped name was two
       characters wide and identity is the whole point of the tile. */
    --cmd-w: 80px;
    /* Half of RecentStrip's 132px face reaches into each bottom seat's inner
       corner, and the seam itself is where combat is read. 5rem clears both
       and leaves a seat's tiles nearer its own plate than its neighbour's. */
    --seam-clear: 5rem;
  }
  /* At the end of the seat's rim opposite the identity plate: the plate is
     pinned to the outer corner (IdentityBar's CORNER map), so left-plate
     seats take the inner end and right-plate seats the outer one — always
     standing off the seam. */
  .side-start {
    align-self: flex-start;
    margin-inline-start: var(--seam-clear);
  }
  .side-end {
    align-self: flex-end;
    margin-inline-end: var(--seam-clear);
  }
</style>
