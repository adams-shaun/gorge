<script lang="ts">
  import { onMount } from 'svelte';
  import { session } from '../lib/session.svelte';
  import { tables } from '../lib/tables.svelte';
  import { pushFeed, type FeedLine } from '../lib/feed';
  import TableCell from '../components/TableCell.svelte';
  import Feed from '../components/Feed.svelte';
  import type { Widget } from '../protocol';

  let feed = $state<FeedLine[]>([]);
  onMount(() => {
    session.ensureOverview();
    tables.load().catch((err: unknown) => console.error('overview: failed to load tables', err));
    return session.stream.onFrame((f) => {
      if (f.t === 'widget' && f.table) feed = pushFeed(feed, { table: f.table, match: f.match ?? 0, seq: f.seq, line: (f.body as Widget).last });
    });
  });
</script>

<main class="overview">
  <section class="grid">
    {#each tables.list as t (t.info.id)}
      <TableCell table={t} />
    {/each}
  </section>
  <aside class="rail"><Feed lines={feed} /></aside>
</main>

<style>
  .overview { display: grid; grid-template-columns: 1fr 22rem; height: 100vh; }
  .grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(18rem, 1fr)); gap: 1rem; padding: 1rem; align-content: start; }
  .rail { border-left: 1px solid #333; overflow: hidden; }
</style>
