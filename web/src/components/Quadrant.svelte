<script lang="ts">
  import type { PlayerView } from '../protocol';
  import { groupBattlefield } from '../lib/board';
  import CardTile from './CardTile.svelte';

  /** Quadrant shows one player's battlefield, split into the three rows board.ts groups it into. It has no rules knowledge: grouping and ordering come entirely from groupBattlefield. */
  let { player, colour }: { player: PlayerView; colour: string } = $props();

  const groups = $derived(groupBattlefield(player.battlefield));
</script>

<div class="quadrant" style:background={`${colour}22`} style:border-color={colour}>
  <div class="row lands">
    {#each groups.lands as c (c.id)}<CardTile card={c} />{/each}
  </div>
  <div class="row creatures">
    {#each groups.creatures as c (c.id)}<CardTile card={c} size="large" />{/each}
  </div>
  <div class="row others">
    {#each groups.others as c (c.id)}<CardTile card={c} />{/each}
  </div>
</div>

<style>
  .quadrant { box-sizing: border-box; width: 100%; height: 100%; border: 1px solid transparent; padding: .5rem; display: flex; flex-direction: column; gap: .4rem; overflow: auto; }
  .row { display: flex; flex-wrap: wrap; gap: .4rem; align-items: flex-end; min-height: 1.5rem; }
  .row.creatures { flex: 1; }
</style>
