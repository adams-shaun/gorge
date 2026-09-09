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
  import HandFan from '../components/HandFan.svelte';
  import PhaseTrack from '../components/PhaseTrack.svelte';
  import { SeatPanelState } from '../lib/seatpanel.svelte';
  import { toneOf } from '../lib/seatpanel.svelte';
  import { optionsByObj, type CardOptions } from '../lib/cardoptions';
  import { loadLogShown, saveLogShown, safeStorage, type LogScope } from '../lib/logshown';
  import { everyVisibleCard, quadrantFor } from '../lib/board';
  import { seatColour } from '../lib/colours';
  import { seatRows } from '../lib/seattable';
  import { buildCardOwnerColour } from '../lib/logrender';
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
  // The log starts hidden for a SEATED player (they are playing, not reading)
  // and stays shown for a spectator (they follow the game through it, and its
  // scrubbing). The choice is persisted per table and per scope — the stops
  // contract, not a component localStorage reach. `liveSeated` is a mount
  // constant, so the scope and the default are fixed for the life of this
  // mounted route. (A finished /m/:match route is a spectator replay and keeps
  // the spectator default.)
  const liveSeated = seated && !finished;
  const logScope: LogScope = liveSeated ? 'seat' : 'spectator';
  let showLog = $state(!liveSeated);
  function toggleLog() {
    showLog = !showLog;
    saveLogShown(safeStorage(), table, logScope, showLog);
  }
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

  // The seated player's own player view — the one whose hand is never
  // redacted (view.go fills Hand for the viewer's seat under every
  // visibility). HandFan renders it as real card faces along the bottom edge;
  // a spectator has no `ownPlayer` and mounts no hand at all.
  const ownPlayer = $derived(
    m.view && seatCtx ? (m.view.players.find((p) => p.seat === seatCtx.seat) ?? null) : null,
  );

  // The board's card-options index (ui21): the pending decision grouped by
  // the object each option concerns. For a seated view this is exactly the
  // decision the seat must answer now — the same decision the seat panel
  // surfaces (SeatPanelState.active), so the board and the panel can never
  // disagree about what this seat is being asked, and both drop it once
  // answered. A spectator has no seat panel and no pending decision, and a
  // seat that has nothing pending gets null — in both cases no tile is
  // marked and no tile carries a menu. Building it once here, from the
  // active decision, is what makes one index serve both 'cards with valid
  // options' and 'valid targets' (R-E4-2: the client only regroups options
  // the server already sent, and posts each one by its own index, R-E4-1).
  const boardOptions = $derived.by((): CardOptions | null => {
    if (!seated || panel === null) return null;
    const d = panel.active;
    if (d === null) return null;
    return {
      byObj: optionsByObj(d),
      picked: [...panel.picked],
      tone: toneOf(d),
      post: (index: number) => panel.click(index),
    };
  });

  // Task 3's colour-coded log: the same name/colour resolution SeatTable's
  // rows use (seats[seat].name falling back to the wire's own player name),
  // off whichever view is currently on screen. Guarded for the pre-snapshot
  // instant when m.view is still null — the template only mounts Transcript
  // once m.view is truthy, but this $derived is read at module scope, not
  // inside that block, so it has to handle null itself.
  const logIdentities = $derived(
    m.view ? seatRows(m.view, m.seats).map((r) => ({ name: r.name, colour: r.colour || seatColour(r.seat, m.seats) })) : [],
  );

  // Task lc1: colour each card name in the log by the colour of the seat
  // that OWNS it — the same colour that seat's name renders in — resolved by
  // the card's object id. The described line carries the id (and the name)
  // but not the owner's colour, so it is looked up from the current view's
  // cards (every visible zone, via everyVisibleCard — which defends the
  // public-spectator null hand) by id -> owner -> that seat's colour. An id
  // absent from the view (a card that has left every visible zone) resolves
  // null and renders uncoloured. Rebuilt each render.
  const logCardColour = $derived(
    m.view
      ? buildCardOwnerColour(
          everyVisibleCard(m.view.players),
          (owner) => m.seats[owner]?.colour || seatColour(owner, m.seats),
        )
      : null,
  );

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
  // The log visibility loads where storage exists (onMount, never SSR), the
  // same discipline as stops. An absent/corrupt value (null) leaves the
  // view-specific default in place.
  onMount(() => {
    const saved = loadLogShown(safeStorage(), table, logScope);
    if (saved !== null) showLog = saved;
  });
</script>

{#if idle}
  <main class="matches-page">
    <MatchList {table} />
  </main>
{:else}
  <main class="table" class:log-hidden={!showLog}>
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
        <Board view={m.view} seats={m.seats} options={boardOptions} />
        {#each m.view.players as p (p.seat)}
          <IdentityBar
            player={p}
            seat={m.seats[p.seat]}
            colour={seatColour(p.seat, m.seats)}
            active={m.view.active === p.seat}
            priority={m.view.priority === p.seat}
            corner={quadrantFor(p.seat, m.view.players.length, m.view.viewer)}
            players={m.view.players}
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
        <!-- The seated player's own hand, as real cards along the bottom edge
             (Task ui17). Only the viewer's own seat ever mounts it; it never
             covers the seat panel's decision UI (panel at the board's TOP,
             hand at the BOTTOM) and only the card faces claim the pointer. -->
        {#if seated && seatCtx && ownPlayer}
          <HandFan player={ownPlayer} />
        {/if}
      </section>
      <aside class="rail"><Rail view={m.view} seats={m.seats} decision={seated ? null : m.decision} emphasizeTop={seated} events={m.dvr.events} showLog={showLog} onToggleLog={toggleLog} /></aside>
      <footer class="transcript" class:hidden={!showLog}>
        {#if !seated}
          <DvrBar dvr={m.dvr} onAction={(a) => m.dispatch(a)} {finished} />
        {/if}
        <div class="log"><Transcript dvr={m.dvr} identities={logIdentities} cardColour={logCardColour} onSeek={seated ? () => {} : (seq) => m.dispatch({ type: 'scrub', seq })} /></div>
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
  /* With the log hidden the last row collapses to nothing and the board takes
     the room, rather than leaving a 10rem empty band across the bottom. */
  .table.log-hidden {
    grid-template-rows: auto 1fr 0;
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
    /* --own-hand-h is the height of the seated player's own hand fan along
       the bottom edge: one card face at HandFan's CARD_W (128px) in the
       63:88 card ratio. It is published here, on the box both the fan and the
       identity bar live in, so "how tall is the hand" is stated once rather
       than duplicated into whatever else needs to keep clear of it. Only the
       seated player's identity bar reads it today (IdentityBar's `bottom`
       corner); a spectator mounts no fan and the fallback is 0. */
    --own-hand-h: calc(128px * 88 / 63);
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
  .transcript.hidden {
    display: none;
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
