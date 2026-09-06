<script lang="ts">
  import { onMount } from 'svelte';
  import { session } from '../lib/session.svelte';
  import { tables } from '../lib/tables.svelte';
  import { pushFeed, type FeedLine } from '../lib/feed';
  import TableCell from '../components/TableCell.svelte';
  import Feed from '../components/Feed.svelte';
  import type { Widget } from '../protocol';

  let feed = $state<FeedLine[]>([]);
  const live = $derived(tables.list.filter((t) => t.info.state === 'live').length);
  onMount(() => {
    session.ensureOverview();
    tables.load().catch((err: unknown) => console.error('overview: failed to load tables', err));
    return session.stream.onFrame((f) => {
      if (f.t === 'widget' && f.table) feed = pushFeed(feed, { table: f.table, match: f.match ?? 0, seq: f.seq, line: (f.body as Widget).last });
    });
  });
</script>

<main class="overview">
  <div class="tables">
    <header class="masthead">
      <h1>gorge</h1>
      <p class="what">
        {live} of {tables.list.length}
        {tables.list.length === 1 ? 'table' : 'tables'} playing
      </p>
    </header>
    <section class="grid">
      {#each tables.list as t (t.info.id)}
        <TableCell table={t} />
      {/each}
    </section>
  </div>
  <aside class="rail"><Feed lines={feed} /></aside>
</main>

<style>
  /*
   * The grid IS the hero: eight games of Magic running at once with their life
   * totals moving. The masthead states what you are looking at and then gets
   * out of the way — a large marketing header above a live board would be
   * decoration competing with the actual subject.
   */
  .overview {
    display: grid;
    grid-template-columns: 1fr minmax(18rem, 22rem);
    height: 100vh;
    background: var(--felt);
  }
  .tables {
    overflow-y: auto;
  }
  .masthead {
    display: flex;
    align-items: baseline;
    gap: var(--sp-3);
    padding: var(--sp-6) var(--sp-4) var(--sp-4);
  }
  h1 {
    margin: 0;
    font-size: var(--t-20);
    font-weight: 600;
    letter-spacing: -0.02em;
  }
  .what {
    margin: 0;
    font-family: var(--font-data);
    font-size: var(--t-12);
    color: var(--ink-dim);
  }
  .grid {
    display: grid;
    grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr));
    gap: var(--sp-3);
    padding: 0 var(--sp-4) var(--sp-4);
    align-content: start;
  }
  .rail {
    background: var(--instrument);
    border-left: 1px solid var(--edge-inst);
    overflow: hidden;
    color: var(--ink-inst);
  }
</style>
