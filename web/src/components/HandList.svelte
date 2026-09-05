<script lang="ts">
  import type { PlayerView } from '../protocol';
  import { visibleHand } from '../lib/board';
  import ManaSymbols from './ManaSymbols.svelte';

  /** HandList is a text list of one player's revealed hand: one line per card, name plus mana symbols, headed with that seat's colour and deck. Rail only mounts this when visibleHand(player) is non-null, but it guards independently too, in case that ever changes. */
  let { player, deck, colour }: { player: PlayerView; deck?: string; colour: string } = $props();

  const hand = $derived(visibleHand(player) ?? []);
</script>

<section class="hand" style:border-left-color={colour}>
  <h3>{player.name}'s hand{#if deck} <span class="deck">· {deck}</span>{/if}</h3>
  {#if hand.length === 0}
    <p class="empty">empty</p>
  {:else}
    <ul>
      {#each hand as c (c.id)}
        <li data-obj={c.id}>
          <span class="name">{c.name}</span>
          {#if c.mana_cost}<ManaSymbols cost={c.mana_cost} />{/if}
        </li>
      {/each}
    </ul>
  {/if}
</section>

<style>
  .hand { margin-bottom: .75rem; border-left: 3px solid transparent; padding-left: .5rem; }
  h3 { margin: 0 0 .25rem; font-size: .8rem; opacity: .8; font-weight: 600; }
  .deck { font-weight: 400; opacity: .8; }
  ul { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: .2rem; }
  li { display: flex; align-items: center; justify-content: space-between; gap: .5rem; font-size: .8rem; }
  .empty { margin: 0; font-size: .8rem; opacity: .5; }
</style>
