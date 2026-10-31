<script lang="ts">
  import { onMount } from 'svelte';
  import { parseRoute, type Route } from './lib/router';
  import { versionWatch } from './lib/versioncheck.svelte';
  import { installConsoleBreadcrumbs } from './lib/breadcrumbs';
  import { getSeat } from './lib/seat';
  // Imported for its side effect: applies the saved/`?fx=` visual-effects mode to <html> at load.
  import './lib/visualfx.svelte';
  import FeedbackButton from './components/FeedbackButton.svelte';
  import Overview from './routes/Overview.svelte';
  import Table from './routes/Table.svelte';
  import Event from './routes/Event.svelte';
  import NewEvent from './routes/NewEvent.svelte';

  let route: Route = $state(parseRoute(location.pathname));
  onMount(() => {
    const onPop = () => (route = parseRoute(location.pathname));
    addEventListener('popstate', onPop);
    // The stale-client watch: the demo redeploys on
    // every merge to main, and a tab opened before a deploy keeps running the
    // old embedded bundle with nothing telling the player. It polls GET / on a
    // slow timer (lib/versioncheck) and flips the banner below when the served
    // index.html names a different bundle than the one this tab was loaded
    // from. It NEVER reloads itself — a reload would drop picked options
    // mid-decision — and it starts nothing until this mount, so no unit test
    // ever polls. Wired at the APP level, not the seat panel: one watch per
    // page, whatever routes and seat panels mount beneath it.
    versionWatch.start();
    const uninstallBreadcrumbs = installConsoleBreadcrumbs();
    return () => {
      removeEventListener('popstate', onPop);
      versionWatch.stop();
      uninstallBreadcrumbs();
    };
  });
</script>

<!-- Mounted outside the route switch: the feedback affordance is available on
     every page, and a route change must not tear down a half-written report. -->
<FeedbackButton
  table={route.kind === 'table' || route.kind === 'match' ? route.table : null}
  seat={getSeat()?.seat ?? null}
/>

<!-- The stale-client banner : non-intrusive by design —
     it overlays nothing and steals no focus; the player reloads when ready. -->
{#if versionWatch.stale}
  <div class="stale-banner" role="status">
    <span>A new version is available — reload to pick it up. Your game is not affected.</span>
    <button type="button" onclick={() => location.reload()}>Reload</button>
  </div>
{/if}

{#if route.kind === 'overview'}
  <Overview />
{:else if route.kind === 'table'}
  <!-- keyed by table: MatchState binds its table/match once at mount, so a route change between two tables must remount, not just update props -->
  {#key `${route.table}/`}
    <Table table={route.table} />
  {/key}
{:else if route.kind === 'match'}
  {#key `${route.table}/${route.match}`}
    <Table table={route.table} match={route.match} />
  {/key}
{:else if route.kind === 'newEvent'}
  <NewEvent />
{:else if route.kind === 'event'}
  <!-- keyed by event: the page binds its seat and stream subscription once at mount -->
  {#key route.event}
    <Event event={route.event} />
  {/key}
{:else}
  <main class="notfound"><h1>Not found</h1><a href="/">Overview</a></main>
{/if}

<style>
  /* The stale-client banner: one quiet strip pinned to the top edge, out of
     every playing surface's way. It is a status (role=status), not an alert —
     nothing is wrong with the game in progress, only with this tab's bundle. */
  .stale-banner {
    position: fixed;
    top: 0;
    left: 50%;
    transform: translateX(-50%);
    z-index: 40;
    display: flex;
    align-items: center;
    gap: var(--sp-3);
    padding: var(--sp-1) var(--sp-3);
    background: var(--instrument-raised);
    color: var(--ink);
    border: 1px solid var(--edge-inst);
    border-top: 0;
    border-radius: 0 0 var(--radius) var(--radius);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    box-shadow: 0 2px 8px rgb(0 0 0 / 0.35);
  }
  .stale-banner button {
    background: var(--offered);
    color: var(--felt-sunk);
    border: 0;
    border-radius: var(--radius);
    padding: 0.15rem var(--sp-2);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    font-weight: 600;
    cursor: pointer;
  }
</style>
