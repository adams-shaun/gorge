<script lang="ts">
  import type { View, EventBody, CardView } from '../protocol';
  import { recentlyMattered, visibleHand } from '../lib/board';
  import CardImage from './CardImage.svelte';

  /** RecentStrip shows the last resolved object large in the board's bottom centre, or nothing — recentlyMattered picks the id, this only resolves it to a CardView already present in the view. */
  let { view, events }: { view: View; events: EventBody[] } = $props();

  function findCard(v: View, obj: number): CardView | null {
    for (const p of v.players) {
      for (const list of [p.battlefield, p.graveyard, p.exile, visibleHand(p) ?? []]) {
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
  /* The last resolved object, lifted off the felt (survey #26). One lift
     token, the same one the inspector uses, so "this is off the surface"
     looks the same wherever it happens. */
  .recent {
    position: absolute;
    /* Large enough to read across the table, small enough not to become a
       lid on it: at the full 220px this sat in the middle of a four-seat
       board and covered the seam where combat happens. */
    --card-w-large: 132px;
    bottom: var(--sp-4);
    left: 50%;
    transform: translateX(-50%);
    z-index: 4;
    pointer-events: none;
    border-radius: var(--radius-card);
    box-shadow: var(--shadow-lift);
  }
</style>
