<script lang="ts">
  import { onMount } from 'svelte';
  import { session } from '../lib/session.svelte';
  import { tables } from '../lib/tables.svelte';
  import { MatchState } from '../lib/match.svelte';
  import Board from '../components/Board.svelte';
  import Rail from '../components/Rail.svelte';
  import IdentityBar from '../components/IdentityBar.svelte';
  import RecentStrip from '../components/RecentStrip.svelte';
  import Transcript from '../components/Transcript.svelte';
  import { quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';

  // match (from the /t/:table/m/:match route) names a specific match; Task 21
  // adds the finished-match/DVR-bar mode that actually replays one. Here,
  // live mode, we still seed MatchState.match with it so a direct link to a
  // still-live match doesn't wait for the next match_start to know its number.
  let { table, match = null }: { table: string; match?: number | null } = $props();
  // App.svelte keys <Table> by `${table}/${match}`, so a route change always
  // remounts a fresh instance — these one-shot reads of table/match are
  // intentional, not a stale-binding bug.
  // svelte-ignore state_referenced_locally
  const m = new MatchState(table);
  // svelte-ignore state_referenced_locally
  if (match !== null) m.match = match;

  onMount(() => {
    const off = session.stream.onFrame((f) => m.apply(f));
    void session.focus(table);
    const t = tables.list.find((x) => x.info.id === table);
    if (t) m.seats = t.seats;
    return () => {
      off();
      void session.unfocus(table);
    };
  });
</script>

<main class="table">
  {#if m.halted}<div class="halted">Table halted: {m.halted}</div>{/if}
  {#if m.view}
    <section class="board">
      <Board view={m.view} seats={m.seats} />
      {#each m.view.players as p (p.seat)}
        <IdentityBar
          player={p}
          seat={m.seats[p.seat]}
          colour={seatColour(p.seat, m.seats)}
          active={m.view.active === p.seat}
          priority={m.view.priority === p.seat}
          corner={quadrantFor(p.seat, m.view.players.length)}
        />
      {/each}
      <RecentStrip view={m.view} events={m.dvr.events} />
    </section>
    <aside class="rail"><Rail view={m.view} seats={m.seats} decision={m.decision} /></aside>
    <footer class="transcript"><Transcript dvr={m.dvr} onSeek={(seq) => m.dispatch({ type: 'scrub', seq })} /></footer>
  {:else}
    <p class="waiting">Waiting for {table}…</p>
  {/if}
</main>

<style>
  .table { display: grid; grid-template-columns: 1fr 18%; grid-template-rows: 1fr 9rem; height: 100vh; }
  .board { position: relative; overflow: hidden; }
  .rail { border-left: 1px solid #333; overflow-y: auto; padding: .5rem; }
  .transcript { grid-column: 1 / -1; border-top: 1px solid #333; overflow-y: auto; font-family: ui-monospace, monospace; font-size: .85rem; }
  .halted { position: absolute; inset: 0 auto auto 0; background: #b00; color: white; padding: .5rem 1rem; z-index: 10; }
  .waiting { padding: 2rem; opacity: .6; }
</style>
