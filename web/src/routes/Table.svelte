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
  import {
    SeatPanelState,
    mulliganPhase,
    toneOf,
  } from '../lib/seatpanel.svelte';
  import { actionable } from '../lib/autopilot';
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
      const state = new SeatPanelState(table, mm, seatCtx);
      // Seed the first SSR/client paint. SeatPanel's effect keeps later views
      // adopted; doing the first one here lets the route place mulligan and
      // the page-level concede control correctly before that child mounts.
      state.adoptView(m.view?.decision ?? null);
      panelCache = { match: mm, state };
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

  // Generic decisions belong to the rail flyout. Mulligan alone keeps the
  // board centre, where the opening hand is the whole task rather than a HUD.
  const mulligan = $derived(panel ? mulliganPhase(panel.active) : null);
  const concede = $derived(panel?.concedeOption ?? null);
  const actionReady = $derived.by(() => {
    const d = panel?.active;
    return !!(d && seatCtx && d.kind === 'priority' && m.view?.priority === seatCtx.seat && actionable(d));
  });
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
      <section class="board">
        <Board view={m.view} seats={m.seats} options={boardOptions} />
        <!-- The table clock is the board's full-width centre band, ringed by
             the ACTIVE seat's one established identity colour.
             It remains display-only for spectators and owns the same stop
             set/callback for a live seat. -->
        <div class="phase-shard" class:mulligan={mulligan !== null}>
          <PhaseTrack
            view={m.view}
            seats={m.seats}
            seat={panel ? seatCtx?.seat ?? null : null}
            stops={panel ? panel.stops : null}
            onToggle={panel ? (step, side) => panel.toggleStop(step, side) : null}
          />
        </div>
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
        {#if seated && seatCtx && m.match !== null && !finished && (mulligan !== null || m.view.over)}
          {#key m.match}
            <SeatPanel view={m.view} seats={m.seats} ctx={seatCtx} table={table} match={m.match} state={panel} />
          {/key}
        {/if}
        <!-- The seated player's own hand, as real cards along the bottom edge
             (Task ui17). Only the viewer's own seat ever mounts it; it never
             covers the seat panel's decision UI (panel at the board's TOP,
             hand at the BOTTOM) and only the card faces claim the pointer. -->
        {#if seated && seatCtx && ownPlayer}
          <!-- ui23: the hand gets the same card-options index the board does,
               so a hand card the pending decision offers something to is
               marked and carries the same options menu (one mechanism, one
               index, one post path). boardOptions is null for a spectator /
               when nothing is pending, so no hand card is marked. -->
          <HandFan player={ownPlayer} options={boardOptions} />
        {/if}
      </section>
      <aside class="rail">
        <Rail view={m.view} seats={m.seats} decision={seated ? null : m.decision} emphasizeTop={seated} events={m.dvr.events} showLog={showLog} onToggleLog={toggleLog} />
        {#if seated && seatCtx && panel && m.match !== null && !finished && mulligan === null && !m.view.over}
          <!-- One owner for generic prompts/options: this rail flyout. The
               board carries only mulligan, so two lists can never disagree. -->
          <div class="action-dock" data-action-dock>
            <button
              class="action-arrow"
              class:ready={actionReady}
              type="button"
              aria-label="Show available actions"
              data-action-arrow
            ><span aria-hidden="true">&lt;&lt;</span><span>Actions</span></button>
            <div class="action-surface" data-action-flyout>
              <div class="surface-controls">
                <button
                  class="pass"
                  type="button"
                  data-pass
                  onclick={() => panel.passClick()}
                  disabled={!panel.passOption || panel.busy}
                >Pass</button>
                <button
                  class="fast"
                  class:on={panel.fastForward}
                  type="button"
                  data-fast-forward
                  aria-pressed={panel.fastForward}
                  onclick={() => { const view = m.view; if (view) { panel.startFastForward(); panel.considerAuto(view); } }}
                  disabled={!panel.active || panel.busy}
                >Fast forward</button>
              </div>
              {#key m.match}
                <SeatPanel view={m.view} seats={m.seats} ctx={seatCtx} table={table} match={m.match} state={panel} placement="flyout" />
              {/key}
            </div>
          </div>
        {/if}
      </aside>
      {#if panel && concede}
        <div class="concede-control">
          {#if panel.confirming}
            <button class="confirm" type="button" data-confirm-concede onclick={() => panel.confirmConcede()} disabled={panel.busy}>
              Concede — confirm
            </button>
          {:else}
            <button type="button" data-concede-control onclick={() => panel.click(concede.index)} disabled={panel.busy}>
              Concede
            </button>
          {/if}
        </div>
      {/if}
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
    grid-template-rows: 1fr 10rem;
    height: 100vh;
    background: var(--felt);
  }
  /* With the log hidden the last row collapses to nothing and the board takes
     the room, rather than leaving a 10rem empty band across the bottom. */
  .table.log-hidden {
    grid-template-rows: 1fr 0;
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
    /* The player's identity and hand are one bottom seat strip. The fixed
       identity bay is consumed by HandFan rather than overlaid on it. */
    --own-seat-w: 12rem;
  }
  .phase-shard {
    position: absolute;
    top: 50%;
    left: 0;
    right: 0;
    transform: translateY(-50%);
    z-index: 7;
    min-width: 0;
  }
  /* Mulligan keeps the centre for its hand; the shard yields upward for that
     one decision instead of competing for the same pixels. */
  .phase-shard.mulligan {
    top: 0;
    transform: none;
  }

  .rail {
    position: relative;
    min-width: 0;
    background: var(--instrument);
    border-left: 1px solid var(--edge-inst);
    overflow: visible;
    color: var(--ink-inst);
  }
  /* Reserve the quiet top-right corner for concede; the rail's ordinary log
     switch stays on the same line without sitting under it. */
  .rail :global(.logbar) {
    padding-right: 5.5rem;
  }
  .action-dock {
    /* One forgiving hover region: the hidden surface, transparent seam bridge
       and << handle are descendants of this box. Losing hover delays closure,
       so a diagonal handle-to-panel path does not retract under the pointer. */
    --dock-close-delay: 180ms;
    position: absolute;
    top: 50%;
    left: 0;
    width: min(26rem, calc(100vw - 17rem - var(--sp-2)));
    transform: translate(-100%, -50%);
    z-index: 8;
  }
  .action-dock::after {
    content: '';
    position: absolute;
    top: calc(-1 * var(--sp-1));
    right: calc(-1 * var(--sp-1));
    bottom: calc(-1 * var(--sp-1));
    width: var(--sp-2);
  }
  .action-arrow {
    position: absolute;
    top: 0;
    left: 100%;
    z-index: 2;
    display: flex;
    justify-content: space-between;
    gap: var(--sp-1);
    width: 6rem;
    border: 1px solid var(--edge-inst);
    border-left: 0;
    border-radius: 0 var(--radius) var(--radius) 0;
    padding: var(--sp-1) var(--sp-2);
    background: var(--instrument);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    cursor: pointer;
  }
  .action-arrow.ready {
    border-right-color: var(--initiative);
    color: var(--initiative);
    font-weight: 600;
  }
  .action-surface {
    opacity: 0;
    visibility: hidden;
    pointer-events: none;
    transform: translateX(var(--sp-2));
    background: var(--instrument);
    border: 1px solid var(--edge-inst);
    border-radius: var(--radius) 0 var(--radius) var(--radius);
    overflow: hidden;
    transition:
      transform 0.14s ease-out var(--dock-close-delay),
      opacity 0.14s ease-out var(--dock-close-delay),
      visibility 0s linear calc(var(--dock-close-delay) + 0.14s);
  }
  .action-dock:hover .action-surface,
  .action-dock:focus-within .action-surface {
    opacity: 1;
    visibility: visible;
    pointer-events: auto;
    transform: translateX(0);
    transition-delay: 0s;
  }
  .surface-controls {
    display: flex;
    justify-content: flex-end;
    gap: 1px;
    padding: 2px;
    border-bottom: 1px solid var(--edge-inst);
  }
  .surface-controls button {
    border: 0;
    border-bottom: 2px solid transparent;
    border-radius: 0;
    padding: var(--sp-1) var(--sp-2);
    background: var(--instrument-raised);
    color: var(--ink-inst);
    font-family: var(--font-ui);
    font-size: var(--t-12);
    cursor: pointer;
  }
  .surface-controls button:disabled {
    color: var(--ink-faint);
    cursor: default;
  }
  .surface-controls .pass:not(:disabled),
  .surface-controls .fast.on {
    border-bottom-color: var(--offered);
  }
  .action-surface :global(.seat-panel.flyout) {
    width: 100%;
    min-width: 0;
    max-height: min(32vh, 24rem);
    border: 0;
    border-radius: 0;
    backdrop-filter: none;
  }

  .concede-control {
    position: fixed;
    top: var(--sp-2);
    right: var(--sp-2);
    z-index: 9;
  }
  .concede-control button {
    padding: var(--sp-1) var(--sp-2);
    border: 1px solid color-mix(in srgb, var(--danger) 42%, var(--edge-inst));
    border-radius: var(--radius);
    background: var(--instrument);
    color: color-mix(in srgb, var(--danger) 68%, var(--ink));
    font-size: var(--t-12);
    cursor: pointer;
  }
  .concede-control button.confirm {
    background: var(--danger);
    color: var(--felt-sunk);
    font-weight: 600;
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
