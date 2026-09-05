<script lang="ts">
  import type { CardView } from '../protocol';
  import CardImage from './CardImage.svelte';

  /**
   * CardTile is the battlefield/stack/strip face of one object. It has no
   * rules knowledge: CardImage draws the face (art, or the text fallback with
   * name/mana symbols/types/P-T); this wrapper only adds per-instance state
   * that is not part of the card's printed identity — tapped, damage,
   * counters, summoning sickness — plus the data-obj anchor arrows use.
   */
  let { card, size = 'tile' }: { card: CardView; size?: 'tile' | 'large' } = $props();
</script>

<div class="card-tile" class:tapped={card.tapped} class:sick={card.summon_sick} data-obj={card.id}>
  <CardImage {card} {size} />
  {#if card.damage > 0}<span class="badge damage">{card.damage}</span>{/if}
  {#if card.counters && Object.keys(card.counters).length}
    <div class="counters">
      {#each Object.entries(card.counters) as [kind, n] (kind)}
        <span class="chip">{n}{kind.slice(0, 1)}</span>
      {/each}
    </div>
  {/if}
</div>

<style>
  .card-tile { position: relative; display: inline-block; transition: transform .15s ease; }
  .card-tile.tapped { transform: rotate(15deg); }
  .card-tile.sick { opacity: .6; }
  .badge {
    position: absolute; top: -.4em; right: -.4em;
    background: #b00020; color: #fff; border-radius: 999px;
    font-size: .65rem; font-weight: 700; padding: .1em .4em; line-height: 1;
  }
  .counters { position: absolute; bottom: -.35em; left: .25em; display: flex; gap: .15em; }
  .chip {
    background: #333; color: #fff; font-size: .6rem; border-radius: 3px;
    padding: 0 .3em; line-height: 1.3;
  }
</style>
