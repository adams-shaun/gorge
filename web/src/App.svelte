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
  <Table table={route.table} />
{:else if route.kind === 'match'}
  <Table table={route.table} match={route.match} />
{:else}
  <main class="notfound"><h1>Not found</h1><a href="/">Overview</a></main>
{/if}
