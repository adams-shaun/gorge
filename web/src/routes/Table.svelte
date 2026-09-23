<script lang="ts">
  import { onMount } from 'svelte';
  import { session } from '../lib/session.svelte';
  import { tables } from '../lib/tables.svelte';
  import { MatchState } from '../lib/match.svelte';
  import BoardStage from '../components/BoardStage.svelte';
  import Arrows from '../components/Arrows.svelte';
  import Rail from '../components/Rail.svelte';
  import IdentityBar from '../components/IdentityBar.svelte';
  import PileHost from '../components/PileHost.svelte';
  import Transcript from '../components/Transcript.svelte';
  import DvrBar from '../components/DvrBar.svelte';
  import MatchList from '../components/MatchList.svelte';
  import ConcedeControl from '../components/ConcedeControl.svelte';
  import RestartControl from '../components/RestartControl.svelte';
  import SeatPanel from '../components/SeatPanel.svelte';
  import HandFan from '../components/HandFan.svelte';
  import {
    SeatPanelState,
    mulliganPhase,
    toneOf,
  } from '../lib/seatpanel.svelte';
  import { optionsByObj, optionsByPlayer, resolveCardFollowUp, type CardOptions } from '../lib/cardoptions';
  import { startRematch } from '../lib/playvsbot';
  import { stuckDecision } from '../lib/prompt';
  import { loadLogShown, saveLogShown, type LogScope } from '../lib/logshown';
  import { safeStorage } from '../lib/storage';
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

  // Generic decisions belong to the clock's ACTIONS tab. Mulligan alone
  // keeps the board centre, where the opening hand is the whole task rather
  // than a HUD. A REQUIRED prompt (brief Jobs 1–2) joins it: a decision with
  // no pass option (toneOf 'initiative') is answered on the board, not in the
  // ACTIONS drop — the mulligan round is the precedent, and the offered
  // priority windows stay in the drop, which is the split the reporter asked
  // for: prompts are not "available actions" and are never hidden behind one.
  const promptPending = $derived(
    panel !== null && panel.active !== null && toneOf(panel.active) === 'initiative',
  );
  const mulligan = $derived(panel ? mulliganPhase(panel.active) : null);
  const concede = $derived(panel?.concedeOption ?? null);

  // The table's own public config (format, bot_policy, mulligans), for the
  // restart control's POST. TableInfo always carries all three.
  const tableInfo = $derived(tables.list.find((t) => t.info.id === table)?.info ?? null);

  // Restart is offered only on the play-vs-bot shape createGame builds:
  // a live seated 2-seat table, exactly one of those seats a real person,
  // and that person is the viewer. A spectator, a finished /m/:match
  // replay, a two-human table and the pre-game mulligan round get none
  // (the mulligan round is not yet a game to restart). Offering it does
  // NOT depend on a concede being pending — the useful moment is any time
  // mid-game. The seat that is the human is seat 0 in createGame's
  // rotation, but the predicate reads the wire flags, not a hard-coded
  // index, so an inverted assignment still finds it.
  const restartable = $derived.by(() => {
    if (!liveSeated || seatCtx === null || m.match === null || mulligan !== null) return false;
    if (m.seats.length !== 2) return false;
    // Exactly ONE human seat, and it is the viewer (a two-human table and a
    // viewer sitting in the bot seat both fail here).
    if (m.seats.filter((s) => s.human).length !== 1) return false;
    return m.seats.findIndex((s) => s.human) === seatCtx.seat;
  });

  let restartConfirming = $state(false);
  let restartBusy = $state(false);
  let restartError = $state<string | null>(null);

  async function restart() {
    if (tableInfo === null) return;
    restartBusy = true;
    restartError = null;
    try {
      // Seat 0 is the human and seat 1 the bot (cmd/gorged's createGame
      // builds Humans: [0] on two seats), so seat 0's deck id is
      // human_deck and seat 1's is bot_deck.
      const join = await startRematch(
        tableInfo.format as 'constructed' | 'commander',
        m.seats[0]?.deck_id ?? '',
        m.seats[1]?.deck_id ?? '',
        tableInfo.bot_policy,
        tableInfo.mulligans,
      );
      window.location.href = join;
    } catch (e) {
      restartError = e instanceof Error ? e.message : String(e);
      restartConfirming = false;
      restartBusy = false;
    }
  }
  // fb-20260917T231628Z: the log show/hide control moved into the OPTIONS drop
  // (PlaySettingsPanel's Layout section) for ordinary seated play. The drop is
  // mounted only when BoardStage's `controls` object is non-null, so the rail's
  // LOGS toggle must survive in exactly the states where the drop does not exist
  // (spectator, mulligan round, game over, the finished /m/:match replay) — a
  // spectator whose saved preference is "hidden" could otherwise never get the
  // log back. The reachability flag is DEFINED AS `controls !== null`, so the
  // rail's toggle and the drop's switch can never both be absent.
  const controls = $derived(
    panel && seatCtx && m.match !== null && m.view !== null && !finished && mulligan === null && !m.view.over
      ? { state: panel, ctx: seatCtx, table, match: m.match, showLog, onToggleLog: toggleLog }
      : null,
  );
  const optionsReachable = $derived(controls !== null);
  // The empty-answer safety net (the Squadron Hawk fail-to-find soft-lock):
  // a decision for THIS seat that carries no options is one no picker can
  // render, so the Pending tray names it — and, when the empty answer is
  // legal (Min 0), carries a Continue that submits it through the seat
  // panel's ordinary posting path (SeatPanelState.continueEmpty). The engine
  // resolves every such shape silently since the empty-choose fix, so a live
  // server should never hold one; the tray entry is belt-and-braces (an older
  // server, a new engine shape) so a pending decision for this seat is never
  // invisible. A spectator has no seat and no answerable decision: the rail's
  // own decision line already names who is being asked.
  const stuck = $derived(seated ? stuckDecision(m.view?.decision ?? null) : null);
  const onContinue = $derived(
    panel !== null && stuck !== null && stuck.answerable ? () => panel.continueEmpty() : null,
  );
  // A direct card action can hand the server a first-stage choice and receive
  // a second decision for the same object (Underground Sea's activate -> Add
  // U / Add B flow; a multi-ability mana source's stage-1 ability pick -> its
  // stage-2 colour wheel, fb-e079def5). Remember only that one network
  // continuation: the picker itself is unmounted while the posted decision is
  // hidden, so it cannot carry open state across the round trip. The effect
  // is the one decoder of the expectation -- resolveCardFollowUp opens the
  // picker only when the next decision really carries 2-6 options on the
  // expected object, and disarms otherwise.
  let expectedCardFollowUp = $state<{ seq: number; obj: number } | null>(null);
  let autoOpenCardDecision = $state<{ seq: number; obj: number } | null>(null);
  //
  $effect(() => {
    const d = m.view?.decision ?? null;
    const expected = expectedCardFollowUp;
    if (d === null || expected === null || d.seq === expected.seq) return;
    autoOpenCardDecision = resolveCardFollowUp(expected, d);
    expectedCardFollowUp = null;
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
      source: d.source,
      byObj: optionsByObj(d),
      byPlayer: optionsByPlayer(d),
      picked: [...panel.picked],
      tone: toneOf(d),
      autoOpenObj: autoOpenCardDecision?.seq === d.seq ? autoOpenCardDecision.obj : undefined,
      post: (index: number, expectFollowUp = false, holdPriority = false) => {
        const obj = d.options.find((option) => option.index === index)?.obj;
        expectedCardFollowUp = expectFollowUp && obj !== undefined ? { seq: d.seq, obj } : null;
        autoOpenCardDecision = null;
        panel.click(index, { holdPriority });
      },
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
    const off = session.stream.onFrame((f) => {
      // MatchState owns the board/DVR half of rewind; the panel owns pending
      // posts and timers. Reset both before the frame's shorter seq space is
      // exposed, so no paced pass from the discarded tail can fire into it.
      if (f.t === 'rewind') panelCache?.state.rewind();
      m.apply(f);
    });
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
        <!-- The table clock is the board's full-width centre lane, ringed by
             the ACTIVE seat's one established identity colour. BoardStage
             reserves that lane in the seat geometry, so no card row continues
             beneath the band. It remains display-only for spectators and owns
             the same stop set/callback for a live seat. -->
        <BoardStage
          view={m.view}
          seats={m.seats}
          options={boardOptions}
          seat={panel ? seatCtx?.seat ?? null : null}
          stops={panel ? panel.stops : null}
          onToggle={panel ? (step, side) => panel.toggleStop(step, side) : null}
          mulligan={mulligan !== null}
          {controls}
        />
        {#each m.view.players as p (p.seat)}
          <IdentityBar
            player={p}
            seat={m.seats[p.seat]}
            colour={seatColour(p.seat, m.seats)}
            active={m.view.active === p.seat}
            priority={m.view.priority === p.seat}
            corner={quadrantFor(p.seat, m.view.players.length, m.view.viewer)}
            players={m.view.players}
            options={boardOptions}
          />
        {/each}
        <!-- The last resolved card's artwork lives in the rail's stack
             section now (fb-20260916T225456Z): Rail renders ResolvedCard
             from the same m.dvr.events it already receives, and the old
             RecentStrip board overlay — absolutely positioned over the
             board's bottom centre, a patchwork of two earlier complaints
             about the same element — is deleted. -->
        <!-- `finished` is the /t/:table/m/:match route: loadFinished paints a
             FROZEN replay of an already-played match, with no stream and no
             session.focus, so view.decision is whatever was pending at that
             point in history and never advances. A seat panel there offers
             buttons that post a long-stale seq, the server rejects every one,
             and the page looks hung while the live game waits elsewhere --
             which is exactly what happened the first time this was played.
             A seat acts only on the live table route. -->
        {#if seated && seatCtx && m.match !== null && !finished && (mulligan !== null || m.view.over || promptPending)}
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
        <!-- The concede control (when a concede option is pending) is passed
             to Rail as a logbar snippet: it renders inside the rail's own
             LOGS row, in normal flex flow at the row's right edge. fb-53bd45b9:
             it used to be absolutely positioned at top: 3rem inside the rail,
             a hand-calibrated offset that cleared the logbar but landed on
             seat row 0 (the stacked zone counts made each row taller), and
             its z-index: 9 painted over the row's life and pile counts —
             eating the pile buttons' clicks in the covered band. In flow
             inside the logbar row, "floating" is structurally impossible:
             the row is the anchor and grows if the control needs height.
             The state stays here (SeatPanelState wiring); only the markup's
             host row moved. -->
        <!-- fb-20260917T231628Z: while the OPTIONS drop is reachable the log
             switch lives there, so the rail renders no second control (its
             own {#if onToggleLog} hides the row's button when the prop is
             null). The drop is NOT mounted for a spectator, the mulligan
             round, a game-over board or the finished replay — exactly the
             states where the rail keeps its toggle. -->
        <Rail
          view={m.view}
          seats={m.seats}
          decision={seated ? null : m.decision}
          {stuck}
          {onContinue}
          emphasizeTop={seated}
          events={m.dvr.events}
          showLog={showLog}
          onToggleLog={optionsReachable ? null : toggleLog}
          yields={panel?.yields ?? null}
          onYield={panel ? (key) => panel.addYield(key) : null}
          viewerSeat={seated ? (seatCtx?.seat ?? null) : null}
          options={boardOptions}
        >
          {#snippet logbar()}
            {#if restartable}
              <RestartControl
                confirming={restartConfirming}
                busy={restartBusy}
                onArm={() => { restartConfirming = true; restartError = null; }}
                onConfirm={() => void restart()}
              />
              {#if restartError}
                <span class="restart-error" role="alert">{restartError}</span>
              {/if}
            {/if}
            {#if panel && concede}
              <ConcedeControl
                confirming={panel.confirming}
                busy={panel.busy}
                onArm={() => panel.click(concede.index)}
                onConfirm={() => panel.confirmConcede()}
              />
            {/if}
          {/snippet}
        </Rail>
      </aside>
      <footer class="transcript" class:hidden={!showLog}>
        {#if !seated}
          <DvrBar dvr={m.dvr} onAction={(a) => m.dispatch(a)} {finished} />
        {/if}
        <div class="log"><Transcript dvr={m.dvr} identities={logIdentities} cardColour={logCardColour} notes={panel?.autoLog ?? []} onSeek={seated ? () => {} : (seq) => m.dispatch({ type: 'scrub', seq })} /></div>
      </footer>
      <!-- fb-20260914T121642Z: the one arrows overlay lives HERE, at the
           table root, not inside the felt subtree. section.board clips its
           own content (overflow: hidden bounds the felt/cards), so an overlay
           mounted under it cannot draw a line that reaches the stack rail —
           which is why every stack-to-stack target arrow (a counterspell's
           arrow to the spell beneath it) was computed but invisible. Hosted
           by the table root the overlay's box contains both endpoints, and
           .table below is its positioned containing block. Still
           pointer-events: none; arrowsFor/previewArrowsFor are unchanged. -->
      <!-- fb-20260916T225802Z: the table's ONE pile modal. The rail's pile
           buttons and the identity bar's graveyard/exile icons open it
           through the shared pileOpener store; PileHost renders the single
           instance and hands it boardOptions, so pile cards are actionable
           exactly where the cards are (tone ring, badge, menu — each item
           posting the option's own wire index). -->
      <PileHost view={m.view} seats={m.seats} options={boardOptions} />
      <Arrows view={m.view} options={boardOptions} />
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
    /* The overlay's containing block: Arrows mounts here (fb-20260914T121642Z)
       and positions itself absolute/inset 0 against this box, so its
       coordinates are measured over the whole table — felt and rail both. */
    position: relative;
    /* The rail's floor is what its content measures: since fb-20260917T232028Z
       the seat summary is one text-line tall and its register — the life
       pill, the four one-line zone counts and the row's own chrome — measures
       ~141px (SeatTable.svelte.test.ts's geometry harness). That now sits
       just under the old binding constraint, the stack tile's 144px art
       column, instead of well under it; both clear 176px. The "Concede —
       confirm" control needs 138px in the logbar row it shares with the
       LOGS toggle. 11rem (176px) leaves the ellipsized seat name ~16px at
       the floor and ~56px at the 15% cap of a 1440px viewport — the name is
       the one thing that flexes; the floor itself is unchanged by the
       one-line redesign. The 15% cap matters more than the floor
       on common viewports: with min 17rem the track was pinned to 17rem on
       every window narrower than ~1510px (18% of the viewport fell below the
       floor), so typical laptops saw the full 17rem whatever the content
       needed. */
    grid-template-columns: 1fr minmax(11rem, 15%);
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
  .rail {
    position: relative;
    min-width: 0;
    background: var(--instrument);
    border-left: 1px solid var(--edge-inst);
    overflow: visible;
    color: var(--ink-inst);
  }
  /* Concede lives INSIDE the logbar row (passed to Rail as a snippet), at
     the row's right edge in normal flex flow — the row is its anchor, so it
     cannot paint over the seat table below the row (fb-53bd45b9: the old
     top: 3rem absolute anchor landed on seat row 0 and its z-index ate the
     row's pile-button clicks), and it cannot read as unanchored. The
     .logbar__extra wrapper in Rail pushes this to the right; nothing here
     is absolutely positioned. The control's markup and styles live in
     ConcedeControl.svelte so the geometry fixture renders the real thing
     (SeatTable.svelte.test.ts measures it there). */

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
  /* The restart POST's error, shown inline in the rail's logbar row beside
     the control that raised it (a 404 when CreateGame is not armed). */
  .restart-error {
    margin-left: var(--sp-2);
    color: var(--danger);
    font-size: var(--t-12);
  }
</style>
