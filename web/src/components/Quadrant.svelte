<script lang="ts">
  import type { PlayerView } from '../protocol';
  import { attachedTo, groupBattlefield, stackIdentical } from '../lib/board';
  import CardStack from './CardStack.svelte';

  /** Quadrant shows one player's battlefield, split into the three rows board.ts groups it into. It has no rules knowledge: grouping and ordering come entirely from groupBattlefield; stackIdentical then collapses interchangeable permanents within a row into one tile with a count (CardStack renders the group). Attachments still come from attachedTo for a group of one — a stacked group has none by the stacking rule. */
  let { player, colour }: { player: PlayerView; colour: string } = $props();

  const battlefieldGroups = $derived(groupBattlefield(player.battlefield));
  const stacks = $derived({
    lands: stackIdentical(battlefieldGroups.lands),
    creatures: stackIdentical(battlefieldGroups.creatures),
    others: stackIdentical(battlefieldGroups.others),
  });
</script>

<div class="quadrant" style:--seat={colour}>
  <div class="row lands">
    {#each stacks.lands as g (g.key)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} />
    {/each}
  </div>
  <div class="row creatures">
    {#each stacks.creatures as g (g.key)}
      <CardStack group={g} size="large" attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} />
    {/each}
  </div>
  <div class="row others">
    {#each stacks.others as g (g.key)}
      <CardStack group={g} attachments={g.cards.length === 1 ? attachedTo(player.battlefield, g.cards[0].id) : []} />
    {/each}
  </div>
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
    border-top: 2px solid var(--seat);
    background: var(--felt-raised);
    padding: var(--sp-2);
    display: flex;
    flex-direction: column;
    gap: var(--sp-2);
    overflow: auto;
  }
  /* Nonlands above, lands below (survey #22), so a board stays parseable as
     it grows. Creatures take the space because they are what combat reads. */
  .row {
    display: flex;
    flex-wrap: wrap;
    gap: var(--sp-2);
    align-items: flex-end;
    min-height: 1.5rem;
  }
  .row.creatures {
    flex: 1;
  }
</style>
