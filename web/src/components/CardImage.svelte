<script lang="ts">
  import type { CardView } from '../protocol';
  import { images } from '../lib/images';
  import ManaSymbols from './ManaSymbols.svelte';

  /**
   * Renders a card's Scryfall image when the lookup resolves, else a plain
   * text card (name, mana cost, types, power/toughness for creatures) with a
   * muted "offline" badge while the image source is backed off. This
   * component has no rules knowledge: it only displays fields already on
   * `card` and asks `images` to resolve `card.printing.name`.
   */
  let { card, size = 'tile' }: { card: CardView; size?: 'tile' | 'large' } = $props();

  let url = $state<string | null>(null);
  let offline = $state(false);

  $effect(() => {
    const name = card.printing.name;
    let cancelled = false;
    url = null;
    images.url(name).then((u) => {
      if (!cancelled) url = u;
    });
    offline = images.offline();
    const poll = setInterval(() => (offline = images.offline()), 1000);
    return () => {
      cancelled = true;
      clearInterval(poll);
    };
  });

  const isCreature = $derived(card.types.includes('Creature'));
</script>

<div class="card-image card-image--{size}">
  {#if url}
    <img src={url} alt={card.name} loading="lazy" />
  {:else}
    <div class="card-image__text">
      {#if offline}<span class="card-image__badge">offline</span>{/if}
      <div class="card-image__name">{card.name}</div>
      {#if card.mana_cost}<div class="card-image__cost"><ManaSymbols cost={card.mana_cost} /></div>{/if}
      <div class="card-image__types">{card.types}</div>
      {#if isCreature}<div class="card-image__pt">{card.power}/{card.toughness}</div>{/if}
    </div>
  {/if}
</div>

<style>
  .card-image {
    display: inline-flex;
    aspect-ratio: 63 / 88;
    border-radius: 6px;
    overflow: hidden;
  }
  .card-image--tile {
    width: 90px;
  }
  .card-image--large {
    width: 220px;
  }
  .card-image img {
    width: 100%;
    height: 100%;
    object-fit: cover;
  }
  .card-image__text {
    width: 100%;
    height: 100%;
    display: flex;
    flex-direction: column;
    gap: 0.25em;
    padding: 0.5em;
    box-sizing: border-box;
    background: #24262b;
    color: #e6e6e6;
    font-size: 0.7em;
    position: relative;
  }
  .card-image__name {
    font-weight: 600;
  }
  .card-image__cost,
  .card-image__types {
    opacity: 0.8;
  }
  .card-image__pt {
    margin-top: auto;
    align-self: flex-end;
    font-weight: 600;
  }
  .card-image__badge {
    align-self: flex-start;
    padding: 0.1em 0.4em;
    border-radius: 3px;
    background: #55585f;
    color: #cfd1d6;
    font-size: 0.85em;
  }
</style>
