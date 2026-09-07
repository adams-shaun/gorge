<script lang="ts">
  import type { PlayerView, StackView } from '../protocol';
  import { commandZoneOf, stackIdsOf } from '../lib/commander';
  import CommanderTile from './CommanderTile.svelte';

  /**
   * CommandArea projects one seat's command zone into the CREATURES row of
   * its own Quadrant (CZ2): one CommanderTile per roster commander, in
   * genesis order (CR 903.6), rendered directly as siblings of the creature
   * CardStacks that already live there — not into a private area of its own.
   *
   * WHY THE PRIVATE AREA IS GONE. It used to be a recessed, sticky band
   * pinned to the seat's own rim at land scale (80px): a commander is a
   * creature, and drawing it smaller than the row it competes with in combat
   * was the defect this component existed to fix. Quadrant.svelte now places
   * `<CommandArea>` as the first children of `.row.creatures`, so a commander
   * tile inherits that row's `--card-w: 104px` exactly the way a CardStack
   * does — same scale, same flex-wrap, same row. Losing the old area also
   * loses its rim-pinned stickiness: a commander now scrolls with the rest of
   * the creature row like any other permanent, which is the direct
   * consequence of no longer having a private layout to pin.
   *
   * This component has NO wrapping element: it is a plain `{#each}`, so the
   * tiles it renders become flex items of whichever row includes it, with
   * nothing of its own to size, position or stack. A seat with no roster
   * (a Constructed game) renders literally nothing — not an empty node, not a
   * frame, not a heading.
   */
  let { player, stack = [] }: { player: PlayerView; stack?: StackView[] } = $props();

  const commanders = $derived(commandZoneOf(player, stackIdsOf(stack)));
</script>

{#each commanders as c (c.commander.id)}
  <CommanderTile status={c} player={player.name} seat={player.seat} />
{/each}
