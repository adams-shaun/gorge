<script lang="ts">
  import type { View, EventBody, CardView } from '../protocol';
  import { recentlyMattered } from '../lib/board';
  import CardImage from './CardImage.svelte';

  /** RecentStrip shows the last resolved object large in the board's bottom centre, or nothing — recentlyMattered picks the id, this only resolves it to a CardView already present in the view. */
  let { view, events }: { view: View; events: EventBody[] } = $props();

  function findCard(v: View, obj: number): CardView | null {
    for (const p of v.players) {
      for (const list of [p.battlefield, p.graveyard, p.exile, p.hand]) {
        const c = list.find((x) => x.id === obj);
        if (c) return c;
      }
    }
    for (const s of v.stack) if (s.card?.id === obj) return s.card;
    return null;
  }

  const card = $derived.by(() => {
    const id = recentlyMattered(events);
    return id === null ? null : findCard(view, id);
  });
</script>

{#if card}
  <div class="recent" data-obj={card.id}>
    <CardImage {card} size="large" />
  </div>
{/if}

<style>
  .recent { position: absolute; bottom: 1rem; left: 50%; transform: translateX(-50%); z-index: 4; pointer-events: none; filter: drop-shadow(0 4px 10px rgb(0 0 0 / 0.5)); }
</style>
