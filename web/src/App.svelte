<script lang="ts">
  import { onMount } from 'svelte';
  import { parseRoute, type Route } from './lib/router';
  import Overview from './routes/Overview.svelte';
  import Table from './routes/Table.svelte';

  let route: Route = $state(parseRoute(location.pathname));
  onMount(() => {
    const onPop = () => (route = parseRoute(location.pathname));
    addEventListener('popstate', onPop);
    return () => removeEventListener('popstate', onPop);
  });
</script>

{#if route.kind === 'overview'}
  <Overview />
{:else if route.kind === 'table'}
  <!-- keyed by table: MatchState binds its table/match once at mount (FL-38), so a route change between two tables must remount, not just update props -->
  {#key `${route.table}/`}
    <Table table={route.table} />
  {/key}
{:else if route.kind === 'match'}
  {#key `${route.table}/${route.match}`}
    <Table table={route.table} match={route.match} />
  {/key}
{:else}
  <main class="notfound"><h1>Not found</h1><a href="/">Overview</a></main>
{/if}
