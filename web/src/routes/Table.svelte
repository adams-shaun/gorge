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
  import DvrBar from '../components/DvrBar.svelte';
  import MatchList from '../components/MatchList.svelte';
  import SeatPanel from '../components/SeatPanel.svelte';
  import { quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import { href, navigate } from '../lib/router';
  import { getSeat } from '../lib/seat';

  // match (from the /t/:table/m/:match route) names a specific, already-played
  // match. Task 21's finished mode replays it end to end via loadFinished:
  // no stream, no session.focus — everything from the JSON GETs.
  //
  // App.svelte keys <Table> by `${table}/${match}` (FL-38), so a route change
  // always remounts a fresh instance — these one-shot reads of table/match
  // are intentional, not a stale-binding bug.
  let { table, match = null }: { table: string; match?: number | null } = $props();
  // svelte-ignore state_referenced_locally
  const finished = match !== null;

  // The seat identity is read once from the join URL (?seat=N&token=…,
  // M2e-3's FL-99). With no seat, every line below is the spectator path
  // it always was (R-E4-4).
  const seatCtx = getSeat();
  const seated = seatCtx !== null;
  // svelte-ignore state_referenced_locally
  const m = new MatchState(table, seatCtx ?? undefined);

  // idle: the table has no live match and none imminent, so the match list
  // is the whole page rather than a strip under a "waiting" placeholder.
  const idle = $derived.by(() => {
    if (finished) return false;
    const state = tables.list.find((t) => t.info.id === table)?.info.state;
    return state === 'idle' || state === 'halted';
  });

  onMount(() => {
    if (match !== null) {
      void m.loadFinished(match);
      return;
    }
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

{#if idle}
  <main class="matches-page">
    <MatchList {table} />
  </main>
{:else}
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
        {#if seated && seatCtx && m.match !== null}
          {#key m.match}
            <SeatPanel view={m.view} seats={m.seats} ctx={seatCtx} table={table} match={m.match} />
          {/key}
        {/if}
      </section>
      <aside class="rail"><Rail view={m.view} seats={m.seats} decision={seated ? null : m.decision} emphasizeTop={seated} /></aside>
      <footer class="transcript">
        {#if !seated}
          <DvrBar dvr={m.dvr} onAction={(a) => m.dispatch(a)} {finished} />
        {/if}
        <div class="log"><Transcript dvr={m.dvr} onSeek={seated ? () => {} : (seq) => m.dispatch({ type: 'scrub', seq })} /></div>
      </footer>
    {:else if finished && m.loadError}
      <div class="load-error">
        <p>Match {match} isn't available on {table} ({m.loadError}).</p>
        <a href={href({ kind: 'table', table })} onclick={(e) => { e.preventDefault(); navigate({ kind: 'table', table }); }}>Back to {table}</a>
      </div>
    {:else if finished}
      <p class="waiting">Loading match {match}…</p>
    {:else}
      <div class="idle-inline">
        <p class="waiting">Waiting for {table}…</p>
        <MatchList {table} />
      </div>
    {/if}
  </main>
{/if}

<style>
  .matches-page { min-height: 100vh; }
  .table { display: grid; grid-template-columns: 1fr 18%; grid-template-rows: 1fr 10rem; height: 100vh; }
  .board { position: relative; overflow: hidden; }
  .rail { border-left: 1px solid #333; overflow-y: auto; padding: .5rem; }
  .transcript {
    grid-column: 1 / -1; border-top: 1px solid #333; display: flex; flex-direction: column;
    font-family: ui-monospace, monospace; font-size: .85rem; overflow: hidden;
  }
  .log { flex: 1; overflow-y: auto; min-height: 0; }
  .halted { position: absolute; inset: 0 auto auto 0; background: #b00; color: white; padding: .5rem 1rem; z-index: 10; }
  .idle-inline { grid-column: 1 / -1; grid-row: 1 / -1; overflow-y: auto; }
  .waiting { padding: 2rem; opacity: .6; }
  .load-error { padding: 2rem; }
  .load-error a { color: #6cf; }
</style>
