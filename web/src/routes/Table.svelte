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
  import PhaseTrack from '../components/PhaseTrack.svelte';
  import { SeatPanelState } from '../lib/seatpanel.svelte';
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

  // One SeatPanelState per match, created here rather than inside SeatPanel
  // so the phase track above the board and the panel share ONE stop set and
  // ONE autopilot. Built in a $derived (not a $effect) because the SSR pass
  // renders the seat surface and effects never run there; the cache keeps a
  // recompute from throwing away the seat's live decision, and only a new
  // match number replaces the instance.
  let panelCache: { match: number; state: SeatPanelState } | null = null;
  const panel = $derived.by(() => {
    const mm = m.match;
    if (!seated || seatCtx === null || mm === null || finished) return null;
    if (panelCache === null || panelCache.match !== mm) {
      panelCache = { match: mm, state: new SeatPanelState(table, mm, seatCtx) };
    }
    return panelCache.state;
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
      <!-- The clock goes across the top of the page, above both registers.
           Inside the felt it would sit on top of the corner identity bars
           and under the seat panel; here it is never occluded, never
           occludes anything, and is the first thing on screen — which is
           what "prominent display of user turn and phase" asks for. It is
           the same band for a spectator, with nothing focusable on it. -->
      <div class="track">
        <PhaseTrack
          view={m.view}
          seats={m.seats}
          seat={panel ? seatCtx?.seat ?? null : null}
          stops={panel ? panel.stops : null}
          onToggle={panel ? (step, side) => panel.toggleStop(step, side) : null}
        />
      </div>
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
        <!-- `finished` is the /t/:table/m/:match route: loadFinished paints a
             FROZEN replay of an already-played match, with no stream and no
             session.focus, so view.decision is whatever was pending at that
             point in history and never advances. A seat panel there offers
             buttons that post a long-stale seq, the server rejects every one,
             and the page looks hung while the live game waits elsewhere --
             which is exactly what happened the first time this was played.
             A seat acts only on the live table route. -->
        {#if seated && seatCtx && m.match !== null && !finished}
          {#key m.match}
            <SeatPanel view={m.view} seats={m.seats} ctx={seatCtx} table={table} match={m.match} state={panel} />
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
  /*
   * Two registers on one grid. The board is felt — warm, dark, card art
   * dominant, chrome absent. The rail is instrument — cooler, denser, hairline
   * structure. The temperature difference between them is the seam, and the
   * seam is the identity (design plan).
   *
   * The transcript spans the full width beneath both, because the log is the
   * one thing that describes the whole table rather than either half of it.
   */
  .matches-page {
    min-height: 100vh;
  }
  .table {
    display: grid;
    grid-template-columns: 1fr minmax(17rem, 18%);
    grid-template-rows: auto 1fr 10rem;
    height: 100vh;
    background: var(--felt);
  }
  /* The clock spans both registers, like the transcript beneath them: it
     describes the whole table rather than either half of it. */
  .track {
    grid-column: 1 / -1;
    min-width: 0;
  }
  .board {
    position: relative;
    overflow: hidden;
  }
  .rail {
    background: var(--instrument);
    border-left: 1px solid var(--edge-inst);
    overflow-y: auto;
    color: var(--ink-inst);
  }
  .transcript {
    grid-column: 1 / -1;
    background: var(--instrument);
    border-top: 1px solid var(--edge-inst);
    display: flex;
    flex-direction: column;
    font-family: var(--font-data);
    font-size: var(--t-12);
    overflow: hidden;
  }
  .log {
    flex: 1;
    overflow-y: auto;
    min-height: 0;
  }
  /* A halted table is the one thing that must interrupt: it is the only
     element in the client allowed to sit over the board. */
  .halted {
    position: absolute;
    inset: 0 auto auto 0;
    background: var(--danger);
    color: var(--felt-sunk);
    font-weight: 600;
    padding: var(--sp-2) var(--sp-4);
    z-index: 10;
  }
  .idle-inline {
    grid-column: 1 / -1;
    grid-row: 1 / -1;
    overflow-y: auto;
  }
  .waiting {
    padding: var(--sp-8);
    color: var(--ink-dim);
  }
  .load-error {
    padding: var(--sp-8);
  }
  .load-error a {
    color: var(--mana-u);
  }
</style>
