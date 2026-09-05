<script lang="ts">
  import type { FeedLine } from '../lib/feed';
  import { SEAT_COLOURS } from '../lib/colours';

  let { lines }: { lines: FeedLine[] } = $props();
  let el: HTMLDivElement | undefined;

  // Deterministic colour per table id (not a seat), independent of arrival order.
  function tagColour(id: string): string {
    let h = 0;
    for (let i = 0; i < id.length; i++) h = (h * 31 + id.charCodeAt(i)) >>> 0;
    return SEAT_COLOURS[h % SEAT_COLOURS.length];
  }

  $effect(() => {
    if (lines.length) el?.scrollTo({ top: el.scrollHeight });
  });
</script>

<div class="feed" bind:this={el}>
  {#each lines as l (`${l.table}:${l.match}:${l.seq}`)}
    <div class="line">
      <span class="tag" style:background={tagColour(l.table)}>{l.table}</span>
      <span class="text">{l.line}</span>
    </div>
  {/each}
</div>

<style>
  .feed { height: 100%; overflow-y: auto; padding: .5rem; display: flex; flex-direction: column; gap: .3rem; }
  .line { display: flex; align-items: baseline; gap: .5rem; font-size: .85rem; line-height: 1.3; }
  .tag { flex: none; color: white; font-size: .65rem; font-weight: 700; padding: .1rem .35rem; border-radius: 4px; }
  .text { overflow-wrap: anywhere; }
</style>
