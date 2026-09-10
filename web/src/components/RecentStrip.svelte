<script lang="ts">
  import { onDestroy } from 'svelte';
  import type { View, EventBody, CardView } from '../protocol';
  import { recentlyMattered, visibleHand } from '../lib/board';
  import CardImage from './CardImage.svelte';

  /** RecentStrip shows the last resolved object large in the board's bottom
   *  centre, or nothing. recentlyMattered's event-count window is a safety
   *  bound, not the actual lifetime: a resolve that happens right before a
   *  long run of decisions with no further events (the human's own turn,
   *  most commonly) stayed inside that window and sat over the board
   *  indefinitely in real time — reported as a stale card parked behind the
   *  hand. So the strip's real clock is RECENT_MS of wall time from when a
   *  given resolve first appears: shownId only resets its timer when the id
   *  itself changes, not on every poll that still reports the same id, so
   *  the card cannot be kept alive by repeated polling. */
  let { view, events }: { view: View; events: EventBody[] } = $props();

  const RECENT_MS = 1000;
  let shownId = $state<number | null>(null);
  let lastSeenId: number | null = null;
  let timer: ReturnType<typeof setTimeout> | null = null;

  function clearTimer(): void {
    if (timer !== null) clearTimeout(timer);
    timer = null;
  }

  $effect(() => {
    const id = recentlyMattered(events);
    if (id === lastSeenId) return;
    lastSeenId = id;
    clearTimer();
    shownId = id;
    if (id !== null) timer = setTimeout(() => { shownId = null; }, RECENT_MS);
  });
  onDestroy(clearTimer);

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

  const card = $derived.by(() => (shownId === null ? null : findCard(view, shownId)));
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
