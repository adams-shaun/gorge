package host

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"runtime/debug"
	"sync"

	"github.com/adams-shaun/gorge/bots/heuristic"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/game"
	"github.com/adams-shaun/gorge/engine/game/gorgegame"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/protocol"
)

const defaultMaxIntents = 400000

// DefaultMaxDecisionsPerTurn is the recommended value for
// Options.MaxDecisionsPerTurn — the one gorged installs for its served
// tables. The number is sized from measurement, not chosen round: the
// largest legitimate single turn measured across the gate population
// (sample-deck and repo-deck matches at 2/4/6/8 seats) is 244 decisions,
// and the observed policy stall ran ~10000 decisions per turn without the
// turn advancing, so 25000 clears the legitimate ceiling by ~100x and is
// swept by that stall class within three stall-turns. See the Options
// field's comment for the full reasoning, including why the unit is a
// count and never a duration.
//
// Note this is NOT installed when the field is 0: zero means "no guard",
// so every caller that does not opt in keeps exactly today's behaviour.
const DefaultMaxDecisionsPerTurn = 25000

// defaultExpectedEvents is the expected-size hint passed to a live match log's
// Reserve: just above the measured top of a real match's event count (~74k for
// the 4-seat repo-deck match), so a typical match never reallocates and the
// biggest observed one grows with at most one doubling. Sized from the profile,
// not from defaultMaxIntents: reserving at the intent cap would hold ~5x too
// much of a typical match's final length live.
const defaultExpectedEvents = 80000

// match is one game on a table: the engine, the intent boundaries and
// turn starts a view request needs, and the outcome. mu guards everything
// below gc: the run loop holds it for the duration of each Submit and its
// bookkeeping; readers (ViewAt, Events, fan-out) hold it for reads only and
// never drive the engine.
type match struct {
	table *table
	k     int
	seed  uint64
	// engine built g and rebuilds its past boards. gc is the config it was
	// built from, without the live log's capacity hint.
	engine game.Engine
	gc     game.Config
	seats  []protocol.SeatInfo
	decks  []string
	// slots is the actual []seat.Seat the current match built in play().
	// Registry methods that must reach a per-seat *HumanSeat (Pending,
	// SubmitIntent, Task M2b-2) go through it. Installed once at the top
	// of play(), never reassigned, so a resolved *HumanSeat stays stable
	// for the match's whole lifetime.
	slots []seat.Seat

	mu sync.RWMutex
	// g is the running game. e is its rules engine, for what only exists on
	// that engine; setGame keeps the two together.
	g game.Game
	e *rules.Engine
	// files is the live match's append-only logs; nil in memory mode and
	// after the match is archived (Task 12).
	files *matchFiles
	// persisted is the number of events confirmed appended to this match's
	// events file at the last successful persist; only that prefix is
	// durable, so a crash or kill records the head over it, never the
	// in-memory tail (fix round 1, burst atomicity). 0 in memory mode.
	persisted int
	// bounds[j] is len(e.L.Events) after j intents: the seq one past the
	// end of the j-th burst. bounds[0] is genesis plus the first Advance.
	bounds []uint64
	// turnStarts is the seq of every TurnChange so far — the DVR's ticks.
	turnStarts []uint64
	snaps      []snapshot // Task 11
	intents    int
	state      string // protocol.Match*
	result     string // "win", "draw" or ""
	winner     *uint8
	head       string
	reason     string // crash reason (Task 13)
	// refusals and fallbacks are BP-06 diagnostics: how many Submits the
	// engine refused and how many times the fallback rungs ran. They reach
	// the crash report and the sidecar only; they never feed an event or an
	// answer, so they cannot change a replay. Guarded by m.mu.
	refusals  int
	fallbacks int
	// rootRefusals is BP-08's honest-root diagnostic: how many bots.EnvSeat
	// decisions this match wanted an Env for and got a REFUSED root (nil
	// engine, RootRefused set) — a dead feed, an underivable pool, a failed
	// deal. The seat still answers (its fallback), so this is a counter
	// only, like refusals above; it reaches the crash report and the sidecar
	// and never an event or an answer. Guarded by m.mu (incremented inside
	// projectNext's exclusive section).
	rootRefusals int
	// undo queues accepted undo requests (Registry.Undo, server/host/undo.go) for the
	// play loop. Its size-one signal channel is only a wakeup: undoQueue's
	// pending count preserves every accepted click, rearming the wakeup after
	// each rewind until the queue is empty. Created at match build, never
	// reassigned, and dies with the match.
	undo *undoQueue
	// seatContexts are the contexts the seats attached with. An undo re-points
	// them at the replayed game. Touched only on the match goroutine.
	seatContexts []*bots.MatchContext
	// ledger is HD-2's per-intent provenance ledger (server/host/divergence_ledger.go):
	// one byte per accepted intent, bit 0 caretaker, bit 1 client-auto. Host
	// metadata like m.snaps — never replay input, never an event. Appended in
	// attempt's exclusive section, truncated beside the other per-intent
	// bookkeeping on undo; its length always equals m.intents. Guarded by m.mu.
	ledger []uint8
	// feeds owns the live observation feed of every non-human bots.EnvSeat on
	// this match (server/host/botenv.go, spec §5.2 Create/Observe/Record). Nil when
	// no seat implements bots.EnvSeat — the allocation-free fast path every
	// pure-bot table takes today — and on an engine that deals no search roots.
	// Installed once at the top of play, after the seats are final, and never
	// reassigned. The match goroutine is its only writer; every read
	// (projectNext's Observe, the Submit section's Record, and the tests'
	// post-match inspection) is under m.mu.
	feeds *matchFeeds
}

// snapshot is a forked game at an intent boundary that began a turn.
type snapshot struct {
	intent int
	seq    uint64 // Task 12: the persisted burst boundary this snapshot lines up with on disk.
	g      game.Game
}

// newMatch is the pre-E3 constructor, still called directly by tests and
// embedders: it builds match k from the table's own deck list (rotated by
// k) with the seeded toss.
func (r *Registry) newMatch(t *table, k int) (*match, error) {
	return r.newMatchWith(t, k, nil)
}

// newMatchWith resolves decks, seeds and builds the game through genesis and
// the first Advance, so the returned match is at intent boundary 0. When
// persistence is on it also opens the match's files, writes the live sidecar
// and appends the genesis events; any error halts the table. A non-nil plan
// replaces the rotated deck list with the series hook's per-seat decks and,
// when plan.Chooser >= 0, poses CR 103.1's starting-player choice to that seat
// in place of the toss winner.
func (r *Registry) newMatchWith(t *table, k int, plan *SeriesGame) (*match, error) {
	c := t.cfg
	engine, err := LookupEngine(r.opts.Engines, c.Engine)
	if err != nil {
		return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
	}
	gc, infos, deckNames, err := r.matchConfig(c, k, plan)
	if err != nil {
		return nil, err
	}
	// Events growEvents was the top allocator in ./server/host (2.87 GB of the test
	// binary's profile: every live match log reallocated ~2x its final length
	// on the way up). Preallocating the live log to just above a real match's
	// event count makes the common case grow without reallocating at all, and
	// the one make costs less than the doubling series it replaces. The value
	// is a hint, sized from the measured top of a real match (the 4-seat
	// repo-deck match runs ~74k events; stays under defaultMaxIntents); a match
	// that overruns it doubles once and is still correct. It is a pure capacity
	// hint on the live log and cannot leak spare capacity to a clone (Clone
	// truncates to len), so the clone-sharing invariant is intact.
	live := gc
	live.ExpectedEvents = defaultExpectedEvents
	g, err := engine.New(live)
	if err != nil {
		return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
	}
	m := &match{table: t, k: k, seed: gc.Seed, engine: engine, gc: gc, seats: infos, decks: deckNames, state: protocol.MatchLive,
		undo: newUndoQueue()}
	m.setGame(g)
	m.bounds = []uint64{uint64(len(m.g.Log().Events))}
	m.turnStarts = turnStartsIn(m.g.Log().Events, 0)
	m.snapshotGenesis()
	if r.opts.Dir != "" {
		m.files, err = openMatchFiles(r.opts.Dir, t.cfg.ID, k, r.opts.Sync)
		if err != nil {
			return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
		}
		if err := writeSidecar(r.opts.Dir, m.sidecar(), r.opts.Sync); err != nil {
			m.files.close()
			return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
		}
		if err := m.files.append(m.g.Log().Events, nil); err != nil {
			m.files.close()
			return nil, fmt.Errorf("host: table %s match %d: %w", c.ID, k, err)
		}
		m.persisted = len(m.g.Log().Events) // genesis is durable once appended
	}
	return m, nil
}

// matchConfig resolves match k of table c: the game's config, each seat's wire
// summary and each seat's deck id.
func (r *Registry) matchConfig(c TableConfig, k int, plan *SeriesGame) (game.Config, []protocol.SeatInfo, []string, error) {
	names := make([]string, c.Seats)
	decks := make([][]*cards.Card, c.Seats)
	sideboards := make([][]*cards.Card, c.Seats)
	deckNames := make([]string, c.Seats)
	// Display player names, independent of the deck: configured per seat (the
	// same slice index a match's seat uses, stable under the +k deck
	// rotation), else the deterministic "Player 1".."Player N". These reach
	// the wire as view.PlayerView.Name and protocol.SeatInfo.Name; the deck
	// identity stays on the engine's Names (event text, replay-hashed) and on
	// SeatInfo.Deck, so a player box can show a name that is a name, not a
	// deck stem.
	playerNames := make([]string, c.Seats)
	for i := 0; i < c.Seats; i++ {
		playerNames[i] = fmt.Sprintf("Player %d", i+1)
		if i < len(c.PlayerNames) && c.PlayerNames[i] != "" {
			playerNames[i] = c.PlayerNames[i]
		}
	}
	// cmds holds each seat's commander indices, parallel to decks: the
	// Deck the loader produced for that seat carries its own commanders,
	// so a commander table's seats get a command zone from their own deck
	// and a constructed table's seats never do.
	cmds := make([][]int, c.Seats)
	archetypes := make([]string, c.Seats)
	infos := make([]protocol.SeatInfo, c.Seats)
	for i := 0; i < c.Seats; i++ {
		dn := c.Decks[(i+k)%len(c.Decks)]
		if plan != nil {
			// A series plan indexes decks by SEAT — the hook decides who plays
			// what this match; no k rotation.
			dn = plan.Decks[i]
		}
		d, err := r.opts.LoadDeck(dn)
		if err != nil {
			return game.Config{}, nil, nil, fmt.Errorf("host: table %s match %d: deck %q: %w", c.ID, k, dn, err)
		}
		if d.Name == "" {
			d.Name = dn
		}
		names[i], decks[i], sideboards[i], deckNames[i], cmds[i] = d.Name, d.Cards, d.Sideboard, dn, d.Commanders
		archetypes[i] = d.Archetype
		infos[i] = protocol.SeatInfo{Name: playerNames[i], Deck: d.Name, Colour: protocol.SeatColours[i%len(protocol.SeatColours)], DeckID: deckNames[i]}
		// Human marks the slots TableConfig.Humans seats with a real person:
		// the wire signal a client's undo control reads (protocol.SeatInfo's
		// doc).
		for _, h := range c.Humans {
			if h == i {
				infos[i].Human = true
				break
			}
		}
	}
	gc := game.Config{Seed: MatchSeed(c.Seed, k), Names: names, PlayerNames: playerNames, Decks: decks, Archetypes: archetypes, Sideboards: sideboardConfig(sideboards), Tokens: r.opts.Tokens, NameUniverse: r.opts.NameUniverse, Mulligans: c.Mulligans, WindowDiagnostics: c.WindowDiagnostics,
		// An engine's own livelock watcher is the same non-terminating-loop
		// protection as this package's per-turn decision guard, one level
		// down: an embedder that opted out of the host guard (the
		// supervised-infinite-loop shape the stall tests drive) must not have
		// the engine crash the match instead at its own 400-event cycle
		// threshold.
		GuardDisabled: r.opts.MaxDecisionsPerTurn == 0}
	// The format the table was configured with is threaded into the engine
	// once, here, so the match's config is the single value both the live
	// game and its replay are built from. A commander table also
	// resolves its starting life — CR 903.6's 40 (TableConfig's
	// 0-means-format-default convention) — and its per-seat commanders from
	// the loaded decks; validate has already rejected a commander table whose
	// deck names no commander, so a seat can never silently play without a
	// command zone.
	switch c.Format {
	case FormatCommander:
		gc.Format = game.FormatCommander
		life := c.StartingLife
		if life == 0 {
			life = 40
		}
		gc.StartingLife = life
		gc.Commanders = make([][]int, c.Seats)
		for i := range gc.Commanders {
			gc.Commanders[i] = append([]int(nil), cmds[i]...)
		}
	default: // FormatConstructed: the zero game.Config, every field stays unset.
	}
	if plan != nil && plan.Chooser >= 0 {
		// The series hook chose the seat posed CR 103.1's
		// starting-player choice in place of the toss winner
		// (game.Config.StartChooser). A fresh int, so the plan can never be
		// aliased into the match's Config.
		chooser := plan.Chooser
		gc.StartChooser = &chooser
	}
	return gc, infos, deckNames, nil
}

// setGame installs g as the match's game, at build or under m.mu.
func (m *match) setGame(g game.Game) {
	m.g, m.e = g, gorgegame.Rules(g)
}

// ensurePaymentActions builds the pending decision's payment plans. A game
// that is no PaymentPlanner has none to build.
func (m *match) ensurePaymentActions() {
	if pp, ok := m.g.(game.PaymentPlanner); ok {
		pp.EnsurePaymentActions()
	}
}

// turnStartsIn lists the seq of every TurnChange in evs[from:].
func turnStartsIn(evs []events.Event, from int) []uint64 {
	var out []uint64
	for _, ev := range evs[from:] {
		if ev.Kind == events.TurnChange {
			out = append(out, ev.Seq)
		}
	}
	return out
}

// locked runs fn under the write lock and releases it even if fn panics —
// a panicking Submit must not leave the mutex held, or crash (which takes
// the lock to record the failure) would deadlock the table.
func (m *match) locked(fn func() error) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return fn()
}

// projection runs a seat's engine reads under the exclusive m.mu projectLive
// and snapshotBody take. Those projections use rules.Derived's per-engine
// scratch walk; so do a v2 seat's identity keys, mapping refs and heuristic
// fallback. Serializing the two is what keeps a live match's fanout from
// racing its own seat. The agent round-trip is never inside this call.
func (m *match) projection(f func()) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f()
}

// afterSubmit records the burst that a successful Submit just produced.
// Called with m.mu held.
func (m *match) afterSubmit(before int) {
	m.intents++
	m.bounds = append(m.bounds, uint64(len(m.g.Log().Events)))
	m.turnStarts = append(m.turnStarts, turnStartsIn(m.g.Log().Events, before)...)
}

// info is the sidecar/wire summary. Called with m.mu held for reading.
func (m *match) info() protocol.MatchInfo {
	return protocol.MatchInfo{Table: string(m.table.cfg.ID), Match: m.k, Seed: m.seed, Seats: m.seats,
		State: m.state, Result: m.result, Winner: m.winner, Head: m.head,
		Events: len(m.g.Log().Events), Turns: m.g.Board().Turn, BotPolicy: m.table.cfg.BotPolicy}
}

// sidecar is the on-disk summary of the match. Called with m.mu held.
// For a crashed match the summary reflects the persisted prefix, not the
// in-memory tail: crash() recorded m.head over it, and Events here is the
// persisted count (fix round 1).
func (m *match) sidecar() sidecar {
	events := len(m.g.Log().Events)
	if m.files != nil && m.state == protocol.MatchCrashed {
		events = m.persisted
	}
	return sidecar{Table: string(m.table.cfg.ID), Match: m.k, Seed: m.seed, Seats: m.seats, Names: m.gc.Names,
		PlayerNames: m.gc.PlayerNames, Decks: m.decks, Spectator: m.table.cfg.Spectator.String(), State: m.state, Result: m.result, Winner: m.winner,
		Head: m.head, Events: events, Turns: m.g.Board().Turn, Reason: m.reason, Mulligans: m.gc.Mulligans,
		NameUniverse:      m.gc.NameUniverse.Len() > 0,
		NameUniverseNames: append([]string(nil), m.g.Board().NameUniverseNames...),
		PaidManaCensus:    !m.gc.DisablePaidManaCensus,
		Format:            Format(m.gc.Format), StartingLife: m.gc.StartingLife, Commanders: m.gc.Commanders, BotPolicy: m.table.cfg.BotPolicy,
		StartChooser: m.gc.StartChooser, Engine: m.table.cfg.Engine, EngineKey: m.gc.Key,
		Refusals: m.refusals, Fallbacks: m.fallbacks, RootRefusals: m.rootRefusals}
}

// defaultSeats is PL-14: one bot per seat, seeded from the match seed.
func defaultSeats(policy string, names []string, seed uint64) []seat.Seat {
	return defaultSeatsWithAutoPayMana(policy, false, false, names, seed, 0, bots.Deps{}, hostedDecisionDeadlineMS, nil)
}

// defaultSeatsWithAutoPayMana builds every table bot with the persisted
// auto-payment and CommonSense settings (bots.Options.AutoPayMana and
// bots.Options.CommonSense — the bots/sanity guard; both are part of the
// table's configuration, so a restart rebuilds the same seats), the
// registry's search parallelism and the embedder's card dependency (BP-13:
// deps rides bots.Options.Deps into every factory, so a policy that reads
// card facts builds on a served table). Keeping the legacy wrapper preserves
// embedders and tests that intentionally exercise the historical manual-mana
// policy.
func defaultSeatsWithAutoPayMana(policy string, autoPayMana, commonSense bool, names []string, seed uint64, searchParallelism int, deps bots.Deps, deadlineMS int, configs map[string]map[string]string) []seat.Seat {
	out := make([]seat.Seat, len(names))
	for i := range names {
		// BP-10: BotSearchParallelism rides bots.Options.SearchParallelism so a
		// search entry folds its parallel worlds with the embedder's flag. The
		// policy wrapper (bot_policy.go NewBotPolicySeatWithAutoPayMana) stays
		// untouched — its public signature is used by cmd/cardfuzz and the
		// tests; the table path builds through bots.New with Seed,
		// AutoPayMana, SearchParallelism, BotDeps and CommonSense, exactly what
		// the wrapper would thread.
		bot, err := bots.New(policy, bots.Options{Seed: seed ^ uint64(i+1), AutoPayMana: autoPayMana, CommonSense: commonSense, SearchParallelism: searchParallelism, Deps: deps, DecisionDeadlineMS: deadlineMS, Config: configs[policy]})
		if err != nil {
			panic(err) // policy was normalized before the table was registered.
		}
		out[i] = bot
	}
	return out
}

// parkedDecision is the outcome of installing (parking) the seat that owns one
// pending decision, done — as the loop's structure now requires — before that
// decision is ever published. For a bot seat the Decision runs synchronously
// in parkSeat and in/err are already resolved; for a human seat park installed
// the answerable slot and answer() blocks until a SubmitIntent (or ctx/timeout
// caretaker) arrives. p is the decision's owner seat, kept for the crash line.
type parkedDecision struct {
	p   state.PlayerID
	hs  *parking
	in  decision.Intent
	err error
	// data is the projected decision this parking was installed for. A human
	// refusal keeps the pending decision alive and re-parks the same data, so
	// the seat can answer it again without re-projecting the engine (D1b).
	// Nil for a bot parking, whose answer is already resolved.
	data *parkedData
	// searchSlot (BP-10, spec §7) marks a decision that parked on the
	// registry's FIFO search-slot gate. When such a decision's seat error
	// surfaces while the table's context is cancelled — the slot wait
	// unblocked by Close, or the search seat unwound by the same
	// cancellation — the play loop records a clean abort: a table being
	// closed is not a crash. Every OTHER seat keeps the historical crash
	// contract (Ruling FL-17, TestCloseCancelsASeatBlockedInDecide): a
	// cancelled plain seat crashes the match. Set only by parkSeat's gated
	// Env branch.
	searchSlot bool
}

// answer returns the parked decision's intent: immediately for a bot, after
// blocking on the human seat's parked slot otherwise — the same intent/error
// pair the old loop got straight out of seat.S Decide.
func (pd *parkedDecision) answer() (decision.Intent, error) {
	if pd.hs != nil {
		in, err := pd.hs.await()
		return decision.CloneIntent(in), err
	}
	return decision.CloneIntent(pd.in), pd.err
}

// waitingHumanSubmit reports whether this parking's answer came from a
// registry submit that is waiting for the engine's verdict (its ack channel).
// Only such an answer can be refused and re-parked for the same human: a
// caretaker or plain Decide answer has no reply channel and no human to ask
// again, so it must take the ordinary ladder/crash path instead of
// re-parking the same refused answer forever.
func (pd *parkedDecision) waitingHumanSubmit() bool {
	return pd.hs != nil && pd.hs.ack != nil
}

// provenance reports HD-2's provenance bits for the decision this parking
// just answered (server/host/divergence_ledger.go): the caretaker bit is the seat's
// caretakerFires delta across the await — the counter's only writer is the
// await's own caretaker path, whose increment happens before it returns —
// and the auto bit is the ?auto flag the accepted submit stamped on the
// slot, copied out by await on receive. A bot seat (hs nil) is never a
// caretaker or an autopilot: both bits false. Call after answer() returned
// an intent the loop is about to submit; on the undo/error paths the loop
// never reads it and appends nothing.
func (pd *parkedDecision) provenance() (caretaker, auto bool) {
	if pd.hs == nil {
		return false, false
	}
	return pd.hs.s.caretakerCount() > pd.hs.fires0, pd.hs.auto
}

// reply delivers the engine's verdict for this parking's answer to a waiting
// registry submit: nil when Submit accepted it, the engine's refusal reason
// when it did not. A bot parking has no waiting submit, and a plain seat
// answer carries no reply channel, so both are no-ops.
func (pd *parkedDecision) reply(err error) {
	if pd.hs != nil {
		pd.hs.reply(err)
	}
}

// releaseUnanswered stops a parking play installed but never awaited from
// accepting an answer or stranding a submit that already claimed its slot. It
// is nil-safe (a match that ended has no parking) and a no-op for a bot
// parking (which has no slot) or one whose verdict was already delivered.
func (pd *parkedDecision) releaseUnanswered() {
	if pd != nil && pd.hs != nil {
		pd.hs.releaseUnanswered()
	}
}

// parkedData is the projected shape of one pending decision, split out of the
// park step so the projection — which touches the live engine — can be held
// under the match's exclusive lock while the seat step runs without it.
// Which of the fields is set follows the dispatch order (see projectNext): a
// plain Seat gets only v; a BoardSeat gets only brd, built from the engine
// under the same lock view.Project would occupy, with no View projected at
// all; an EnvSeat that wants the decision gets BOTH v and brd (env.View is
// the plain Seat's projection, env.Board the BoardSeat's board) plus env,
// the honest root the seat answers from.
type parkedData struct {
	p       state.PlayerID
	v       view.View
	dc      decision.Decision
	brd     botpolicy.Board
	isBoard bool
	// env, when non-nil, is the host-built input for this decision's
	// bots.EnvSeat actor (BP-08, spec §5.1): the projected View, the
	// actor's board and the honest root (env.Search.Engine may be nil when
	// the redeal refused; env.RootRefused says why). Set only by the Env
	// branch of projectNext, under the lock; consumed by parkSeat's
	// DecideEnv call, outside it, and never read after the next decision
	// refills brd (the same ownership contract the Board path states).
	env *bots.Env
	// wantsSearchSlot (BP-10, spec §7) is set by envData for a decision whose
	// table-policy entry has Search set: parkSeat takes one of the registry's
	// FIFO search slots around this DecideEnv call (and returns ctx.Err()
	// instead of deciding when the wait is cancelled). Set only by the Env
	// branch of projectNext, like env above.
	wantsSearchSlot bool
}

// projectNext reads the engine's current pending decision and projects the
// board for it, copying options exactly as the old loop did (Ruling FL-19
// minor: *d aliases the engine's pending, and dc.Options a slice header into
// the same backing array, so the copy is required). It touches only the
// engine — never a seat — so the loop can hold it under m.mu.Lock. That is
// the fix's lock discipline: view.Project mutates the engine's Derived cache
// (engine/rules/layers.go Engine.active), so projecting the live engine must not run
// concurrently with any other live projection — a focus subscriber's own
// snapshot build now also takes m.mu exclusively (through projectLive, see
// fanout.go), so the two can never overlap; running it inside the Submit's
// exclusive section keeps this projection on the same side of the rule.
//
// seats lets it type-assert the deciding seat: a seat that implements
// heuristic.BoardSeat gets a botpolicy.Board built from the engine (under the same
// exclusive lock) instead of a projected View, so a bot seat never pays for
// cardViews' string round-trip. The deciding seat is stable for the match, so
// this agrees with parkSeat's own assertion on the same seat.
//
// brd is the per-match botpolicy.Board the host loop keeps and refills for
// every BoardSeat decision (Task d2): BoardFromGameInto clears and reuses its
// three maps instead of allocating them per call — the measured bulk of
// BoardFromGame's cumulative in ./server/host. The ownership contract is in
// botpolicy.BoardFromGameInto's doc comment: brd's maps are built here under
// the match lock, consumed by the answer's DecideBoard outside it, and never
// read after the next decision refills them.
//
// Returns nil when there is no pending decision (game over, or a stall the
// caller resolves via G.Over). Call on the match goroutine, under m.mu.
func projectNext(m *match, seats []seat.Seat, brd *botpolicy.Board) *parkedData {
	// Observe before the pending-decision nil check: a feed captures one frame
	// at every decision boundary, including the final one after the last
	// Submit where the engine has no pending decision left (game over). That
	// makes the live feed the exact mirror of searchseat.RebuildFeed, which
	// visits once at n as well (§5.2 Observe; BP-07's TestHostFeedEqualsRebuildFeed).
	m.feeds.observe(m.e)
	return projectNextData(m, seats, brd)
}

// projectNextData is projectNext without the feed observe — the rewind
// path's entry (BP-09): the feeds the rewind just rebuilt (server/host/undo.go)
// already carry the rewound decision's boundary frame — RebuildFeed's visit
// at n IS that frame, pinned equal to the live feed truncated to the
// boundary (§5.2) — so projecting it again would append a duplicate empty
// burst and shift every later frame index off RebuildFeed's by one, forever.
// Everything below the observe is shared with the ordinary path unchanged:
// one body, two entries. Call on the match goroutine, under m.mu.
func projectNextData(m *match, seats []seat.Seat, brd *botpolicy.Board) *parkedData {
	d := m.g.Pending()
	if d == nil {
		return nil
	}
	_, isHuman := seats[d.Player].(*HumanSeat)
	wantsPlans := isHuman && m.table.cfg.AutoMana
	if consumer, ok := seats[d.Player].(seat.PaymentPlanConsumer); ok && consumer.WantsPaymentActions() {
		wantsPlans = true
	}
	if wantsPlans {
		m.ensurePaymentActions()
	}
	if c, ok := seats[d.Player].(seat.TargetHintConsumer); ok && c.WantsTargetHints() {
		if pp, ok := m.g.(game.PaymentPlanner); ok {
			pp.EnsureTargetHints()
		}
	}
	dc := *d.Clone()
	// Payment plans are an opt-in human interface. Keep the engine's pending
	// decision intact for replay and independently configured bots, but never
	// publish the extension to a human seat when this table has it disabled.
	// Options are untouched, so this is precisely the legacy manual path.
	if isHuman && !m.table.cfg.AutoMana {
		dc.PaymentActions = nil
	}
	// A BoardSeat answers from a botpolicy.Board and needs no projected View:
	// build the Board from the engine the way BoardFromGame reads it (same
	// zones, same derived P/T and keywords the View would carry) and skip
	// view.Project entirely — cardViews' string encode/parse round-trip is the
	// measured bulk of the suite's allocations, and a bot is the seat that
	// does it per decision.
	//
	// The HumanSeat test comes FIRST, and must, because parkSeat tests in that
	// order too: it hands a HumanSeat pd.v before it ever considers BoardSeat.
	// *HumanSeat does not implement BoardSeat today, so the two orders agree
	// either way — but if one ever gained a DecideBoard method, the opposite
	// order here would build only the Board and then park the human on a zero
	// View, blanking a live player's board with nothing failing. Testing the
	// same thing first in both places makes that unrepresentable rather than
	// merely unlikely.
	// BP-08 (spec §5.1): an EnvSeat that wants this decision is answered
	// from a host-built bots.Env, built by m.envData (server/host/botenv.go) — the
	// second entry in the dispatch order argued at the BoardSeat comment
	// below and at parkSeat: HumanSeat, then EnvSeat && WantsEnv, then
	// BoardSeat, then View. The honest root is built HERE, under the match's
	// exclusive lock, from the feed this very projectNext just observed
	// (f.History's last frame is this decision's boundary) and from m.seed
	// and the decision's seq; DecideEnv runs outside it, in parkSeat. The
	// !isHuman guard mirrors parkSeat's HumanSeat-first order: a HumanSeat
	// never implements bots.EnvSeat today, but if one ever did, the human
	// must still be parked on a View, not silently handed an Env.
	if es, ok := seats[d.Player].(bots.EnvSeat); ok && !isHuman && es.WantsEnv(&dc) {
		return m.envData(d.Player, &dc, brd, es)
	}
	if _, ok := seats[d.Player].(heuristic.BoardSeat); ok && !isHuman {
		return &parkedData{
			p:       d.Player,
			dc:      dc,
			brd:     botpolicy.BoardFromGameInto(m.g.Board(), m.g.Chars(), d.Player, brd).Detached(),
			isBoard: true,
		}
	}
	v := view.ProjectForControlled(m.g.Board(), m.g.Chars(), d.Player, view.Seat, controlledSeats(m.g.Board(), d.Player), &dc)
	// The seat's view is built at head, so its round is the exact round-trip
	// count (view.RoundOf over the live log), not the snapshot-only roundOf
	// approximation Project fills in (ui13). A human seat renders this view,
	// so it must agree with the board clock the same match fans out.
	v.Round = view.RoundOf(m.g.Board(), m.g.Log().Events)
	return &parkedData{p: d.Player, v: v, dc: dc}
}

// controlledSeats returns the seats the viewer controls RIGHT NOW under a CR
// 723 player-controlling effect: at most one, the active seat, and only while
// that control is live (state.Game.LiveController -- the controlled player's
// next turn, CR 723.1), nil otherwise. A grant folded on the controller's own
// turn widens nothing until the controlled turn begins (cr-g57). d.Player was
// already rewritten to the controller by rules' controlPlayerRedirect, which
// reads the same predicate, so the seat answering the controlled turn's
// decisions is exactly the seat this widens. Bots are deliberately not
// widened: a BoardSeat answers from botpolicy.Board, which reads the engine
// directly and is built before this point, so control only affects the
// projected human View. That is documented, not incidental.
func controlledSeats(g *state.Game, viewer state.PlayerID) []state.PlayerID {
	if ctl, ok := g.LiveController(g.Active); ok && ctl == viewer {
		return []state.PlayerID{g.Active}
	}
	return nil
}

// parkSeat installs the answerable slot for a projected decision: for a
// HumanSeat it installs the slot without blocking, for any other seat (a bot,
// or an embedder's blocking seat) it calls Decide — a BoardSeat via
// DecideBoard, matching the half projectNext built. It is NEVER called under
// m.mu — a blocking seat must not hold the match mutex across a Decide — but
// by the time play calls it the next decision is already fully projected, and
// the caller publishes (fanout) only after it returns, so publish-outranks-park
// stays closed. The decision's owner seat is stable, so the BoardSeat/HumanSeat
// assertions here match projectNext's, and exactly the field that was built is
// consumed.
func parkSeat(ctx context.Context, seats []seat.Seat, pd *parkedData, undo <-chan state.PlayerID, gate *searchSlots, stop <-chan struct{}) *parkedDecision {
	if hs, ok := seats[pd.p].(*HumanSeat); ok {
		return &parkedDecision{p: pd.p, hs: hs.park(ctx, pd.v, pd.dc, undo), data: pd}
	}
	// BP-08 (spec §5.1): the EnvSeat answers from the host-built Env, OUTSIDE
	// the match lock — DecideEnv may run a whole search, and a blocking seat
	// must never hold m.mu across it. envData set pd.env exactly when the
	// seat asserted as bots.EnvSeat here, so the assertions below agree the
	// same way projectNext's and parkSeat's BoardSeat assertions do; a
	// mismatched pair falls through to the plain Decide path rather than
	// panicking, as the BoardSeat branch does.
	if pd.env != nil {
		if es, ok := seats[pd.p].(bots.EnvSeat); ok {
			// BP-10 (spec §7): a searched decision queues on the registry's
			// FIFO search-slot gate around DecideEnv — never under m.mu (the
			// wait can last a whole search), never timed out and never
			// degraded to another policy. On ctx cancellation the wait returns
			// ctx.Err() and the play loop aborts the match rather than
			// crashing (a table being closed is not a crash). defer releases
			// the slot on every path, panics included.
			if pd.wantsSearchSlot && gate != nil {
				if aerr := gate.acquire(ctx, stop); aerr != nil {
					return &parkedDecision{p: pd.p, err: aerr, searchSlot: true}
				}
				defer gate.release()
				cctx, cancel := decisionCtx(ctx, seats[pd.p])
				in, err := es.DecideEnv(cctx, *pd.env, pd.dc)
				cancel()
				return &parkedDecision{p: pd.p, in: in, err: err, searchSlot: true}
			}
			cctx, cancel := decisionCtx(ctx, seats[pd.p])
			in, err := es.DecideEnv(cctx, *pd.env, pd.dc)
			cancel()
			return &parkedDecision{p: pd.p, in: in, err: err}
		}
	}
	if bs, ok := seats[pd.p].(heuristic.BoardSeat); ok && pd.isBoard {
		in, err := bs.DecideBoard(ctx, pd.brd, pd.dc)
		return &parkedDecision{p: pd.p, in: in, err: err}
	}
	cctx, cancel := decisionCtx(ctx, seats[pd.p])
	in, err := seats[pd.p].Decide(cctx, pd.v, pd.dc)
	cancel()
	return &parkedDecision{p: pd.p, in: in, err: err}
}

// play drives m to completion, abort or crash on the table's goroutine and
// returns the final match state. A panic anywhere in a decision or Submit
// is a crash (spec D15), never a dead goroutine.
//
// ctx is the table's own context (run derives it once from t.stop, over the
// table's whole lifetime, not per match): it is the only cancellation path
// into a Seat.Decide call once the loop is blocked inside one, since t.stop
// itself is polled only between decisions (Ruling FL-17). A bot ignores ctx
// and never blocks; a disconnected human seat is expected to select on it.
func (r *Registry) play(ctx context.Context, t *table, m *match) (final string) {
	defer func() {
		if p := recover(); p != nil {
			// A volume LivelockError (the armed object cap, CV-8) is not an
			// engine bug: the game's board grew past its budget mid-turn,
			// exactly the shape the per-turn decision guard's own stall
			// crash records. Record it through the same ordinary MatchCrashed
			// path with the stall wording -- the table halts with a reason an
			// operator reads as a stall, never as a "panic:" bug report, and
			// it cannot wedge: play returns, run() ends the table.
			if lle, ok := p.(*rules.LivelockError); ok && rules.IsVolumeReason(lle.Reason) {
				final = r.crash(t, m, fmt.Errorf("stalled: %v", lle))
				return
			}
			final = r.crash(t, m, fmt.Errorf("panic: %v\n%s", p, debug.Stack()))
		}
	}()
	// Task M2c-1: if an embedder observes bursts, deliver the genesis burst
	// first so the sink sees the whole chain from its first event — genesis
	// goes through the same observeBurst as every Submit burst, on the match
	// goroutine under m.mu (genesis has no intent, so in is nil). A genesis
	// observation error crashes the match exactly as a per-burst one would
	// (D15).
	if r.opts.OnBurst != nil {
		if err := m.locked(func() error { return r.observeBurst(t, m, 0) }); err != nil {
			return r.crash(t, m, err)
		}
	}
	// AutoMana is the table-level feature gate. A disabled table must keep the
	// pre-payment-plan behaviour for every participant, including bots and a
	// human's timeout caretaker; -bot-auto-mana only takes effect when the
	// feature itself is enabled for the table.
	autoPayMana := t.cfg.autoPayManaEnabled()
	// agent-20261001T041445Z: a determinism harness comparing two runs of one
	// seed must never arm the hosted wall-clock budget — a deadline bail-out
	// answers from the non-searched fallback, so the two runs diverge under
	// load (BP-07 §7). Served tables keep hostedDecisionDeadlineMS unchanged.
	budget := hostedDecisionDeadlineMS
	if r.opts.BotUnboundedDecisions {
		budget = 0
	}
	seats := defaultSeatsWithAutoPayMana(t.cfg.BotPolicy, autoPayMana, t.cfg.BotCommonSense, m.gc.Names, m.seed, r.opts.BotSearchParallelism, r.opts.BotDeps, budget, r.opts.BotConfig)
	if len(t.cfg.SeatBotPolicies) > 0 {
		// E1-2: a table with per-seat bot policies rebuilds its bots from
		// SeatBotPolicies (seat i plays seatBotPolicy(i)); the embedder's
		// r.opts.Seats override below still wins over it.
		seats = seatPolicySeats(t.cfg, autoPayMana, m.gc.Names, m.seed, r.opts.BotSearchParallelism, r.opts.BotDeps, budget, r.opts.BotConfig)
	}
	if r.opts.Seats != nil {
		seats = r.opts.Seats(m.gc.Names, m.seed)
	}
	// Task M2c-2: honor the TableConfig.Humans plan — every listed slot is a
	// real person, so replace the bot that Options.Seats built for it (by
	// default defaultSeats, one bot per seat) with a fresh HumanSeat. The
	// remaining slots stay exactly what Options.Seats produced, so a pure-bot
	// table (Humans nil) is byte-identical to today. Each HumanSeat is armed
	// with its deterministic caretaker a few lines below, exactly like the
	// M2b-5 seats an embedder builds itself through the Seats option.
	for _, h := range t.cfg.Humans {
		seats[h] = NewHumanSeat()
	}
	m.mu.Lock()
	m.slots = seats
	// Create the per-seat observation feeds once the seats are final (after
	// the Humans replacement above, which can swap a bot for a HumanSeat and
	// so must precede the EnvSeat scan). A table with no EnvSeat, or on an engine
	// that deals no search roots, gets nil, and every Observe/Record below is a
	// nil-receiver no-op (§5.2 Create).
	if m.g.Caps().SearchRoots {
		m.feeds = newMatchFeeds(seats)
	}
	m.mu.Unlock()
	// Decklist privacy (server/host/closedlist.go): at a table with a human, every
	// search seat must close its opponents' lists, checked on the seats
	// actually built rather than on the policy's declaration.
	if err := closedListGuard(t.cfg, seats); err != nil {
		return r.crash(t, m, err)
	}
	// Task M2b-3: arm every human seat with its think budget and its
	// deterministic caretaker bot — the one defaultSeats would have built
	// for that slot (seed ^ slot+1), so a timed-out human decision is
	// answered by exactly the intent a pure-bot game would have logged for
	// that seat, keeping the replay byte-identical (D3). The one exception is
	// a payment plan that pays life: a caretaker never auto-selects it (spec
	// §6; newCaretakerSeat), since a human confirms life payments in the
	// client. Done here, once, on the match goroutine before the loop, so it
	// never races a Decide.
	for i, s := range seats {
		if hs, ok := s.(*HumanSeat); ok {
			caretaker, err := newCaretakerSeat(t.cfg.BotPolicy, m.seed^uint64(i+1), autoPayMana, t.cfg.BotCommonSense, r.opts.BotDeps, r.opts.BotConfig[t.cfg.BotPolicy])
			if err != nil {
				return r.crash(t, m, err)
			}
			hs.configure(r.opts.ThinkTimeout, caretaker)
		}
	}
	// Give a seat that needs match-level context (an external agent, a
	// replaying adapter) one chance to attach before the first decision, and
	// report the terminal result once after play returns. A seat that does not
	// implement bots.MatchSeat/MatchEnd is unaffected.
	gameID, secret := matchIdentity(m.seed, m.k)
	defer func() {
		result := bots.MatchResult{Reason: m.reason, DecisionCount: uint64(m.intents)}
		switch m.state {
		case protocol.MatchFinished:
			result.Classification = "natural"
			if m.result == "draw" {
				result.Outcome = "draw"
			} else if m.winner != nil {
				result.Outcome = fmt.Sprintf("p%d_win", *m.winner)
				result.Winner = fmt.Sprintf("p%d", *m.winner)
			}
		case protocol.MatchAborted:
			result.Classification = "halted"
			result.Reason = "aborted"
		default:
			result.Classification = "halted"
		}
		for _, s := range seats {
			bots.EndMatch(s, result)
		}
	}()
	for i, s := range seats {
		mc := &bots.MatchContext{
			Context:      ctx,
			Game:         m.g,
			Engine:       m.e,
			Projection:   m.projection,
			GameID:       gameID,
			Seat:         i,
			Seats:        len(seats),
			Format:       tableFormatName(m.gc.Format),
			Seed:         []byte(fmt.Sprintf("%d/%d", m.seed, m.k)),
			Secret:       secret,
			OwnDeck:      m.g.Chars().OwnDeck(state.PlayerID(i)),
			Mulligan:     mulliganName(m.gc.Mulligans),
			StartingSeat: int(m.g.Board().Active),
			DeadlineMS:   budget,
			Logf: func(format string, args ...any) {
				log.Printf("match %s seat %d: "+format, append([]any{gameID, i}, args...)...)
			},
		}
		if t.cfg.AllowDecklistBots && len(seats) == 2 {
			mc.OpponentDeck = m.g.Chars().OwnDeck(state.PlayerID(1 - i))
			mc.OpponentVisible = mc.OpponentDeck != nil
		}
		m.seatContexts = append(m.seatContexts, mc)
		if err := bots.AttachMatch(s, mc); err != nil {
			return r.crash(t, m, fmt.Errorf("seat %d: attach match: %w", i, err))
		}
		if hs, ok := s.(*HumanSeat); ok {
			if r.opts.TapHuman != nil {
				tap, err := r.opts.TapHuman(t.cfg.ID, m.k, autoPayMana, mc)
				if err != nil {
					return r.crash(t, m, fmt.Errorf("seat %d: build tap: %w", i, err))
				}
				if tap != nil {
					hs.SetTap(tap)
				}
			}
			if err := hs.attachTap(mc, autoPayMana); err != nil {
				return r.crash(t, m, fmt.Errorf("seat %d: attach tap: %w", i, err))
			}
		}
	}
	maxIntents := r.opts.MaxIntents
	if maxIntents == 0 {
		maxIntents = defaultMaxIntents
	}
	// Task HW1: the per-turn progress guard. lastTurn is the turn the game
	// was in when the previous decision was answered; decisionsThisTurn is
	// how many decisions have been answered since the last turn advance. A
	// turn that ADVANCES resets the count — a legitimately long turn of
	// hundreds of decisions keeps passing — while a turn whose decisions
	// never advance it (seed 175's 20000-intent equip loop) accumulates
	// until it trips the limit and the match crashes with a stall reason,
	// halting the table through run()'s ordinary MatchCrashed path. The
	// limit is a count of DECISIONS, never a duration: see
	// Options.MaxDecisionsPerTurn for why the determinism the engine is
	// built on forbids a wall clock here. 0 — the zero value — disables the
	// guard entirely.
	lastTurn := m.g.Board().Turn
	decisionsThisTurn := 0
	perTurnLimit := r.opts.MaxDecisionsPerTurn
	// BP-10 (spec §7): the registry's FIFO search-slot gate. Non-nil only
	// when the registry bounds searched decisions (Options.SearchSlots) AND
	// this table's policy entry has Search set; parkSeat takes it around
	// DecideEnv only for decisions that carry a search Env (parkedData's
	// wantsSearchSlot, set by envData).
	gate := r.searchGateFor(t)
	// parked is the decision currently awaiting its answer (nil before the
	// first live iteration). It is parked — installed, accept-ready — before
	// any fan-out that could publish it, so no decision is ever visible before
	// its seat can accept an answer. The first decision is parked on the first
	// live iteration, after the stop/over checks, so that if parking it blocks
	// (a seat that never answers, ctx-cancelled by Close) and that seat then
	// errors, the error is surfaced as a crash rather than masked by a stop
	// abort that raced the first park.
	var parked *parkedDecision
	// Release a parking the loop installed but never awaited when play exits
	// for any reason other than the normal answer path: the t.stop abort, the
	// maxIntents guard, a crash, or a panic outside attempt. A submit that
	// claimed that slot during the pace window is waiting for a verdict only
	// this goroutine could produce; without this it waits forever even though
	// Close has returned (D2). A parking whose verdict was already delivered
	// is left alone: closing abandoned would let an accepted answer be
	// reported as "no pending decision". Deferred after the recover defer, so
	// it runs first and releases the parking before the crash is recorded.
	defer func() { parked.releaseUnanswered() }()
	var before int
	// Task d2 (buffered reuse): one botpolicy.Board for the whole match,
	// refilled for every decision a BoardSeat answers. projectNext fills it
	// under the match's exclusive lock via BoardFromGameInto (shared maps,
	// cleared-and-reused, no per-decision map allocation); parkSeat consumes
	// it in DecideBoard outside the lock; it is never read after the next
	// decision's refill (botpolicy.BoardFromGameInto's doc comment states the
	// contract, and botpolicy.TestBoardOwnership pins that Decide never
	// retains the Board). Sized to len(m.g.Board().Players) like BoardFromGame's own
	// Life map, once per match — three small maps, nothing per decision.
	brd := botpolicy.NewBoard(len(m.g.Board().Players))
playLoop:
	for n := 0; ; n++ {
		select {
		case <-t.stop:
			return r.abort(m)
		default:
		}
		// Atomically choose an already-accepted undo before committing a
		// natural end. Undo reserves its request under this same match lock,
		// so exactly one side wins: a request queued first is serviced even
		// when the preceding burst ended the game; a finish committed first
		// makes Undo return a conflict instead of 204-then-nothing. This check
		// is the first operation at every post-burst loop boundary, including
		// the boundary after the pace sleep.
		req, undo, final := r.settleNaturalEndOrUndo(t, m)
		if final != "" {
			return final
		}
		if undo {
			// The rewind itself is server/host/undo.go's in-place truncate. A parked
			// decision this path abandons must stop being answerable BEFORE the
			// game is rebuilt, or a submit racing the rewind could be validated
			// against a decision the match no longer asks and silently dropped.
			parked.abandon()
			var err error
			parked, err = r.serviceUndo(ctx, t, m, seats, &brd, req)
			if err != nil {
				return r.crash(t, m, err)
			}
			// continue executes the for-loop post statement (n++), so seed n
			// one behind the rewound intent count. At the next body entry n once
			// again equals the number of intents already submitted.
			n, lastTurn, decisionsThisTurn = m.intents-1, m.g.Board().Turn, 0
			continue
		}
		if n >= maxIntents {
			return r.crash(t, m, fmt.Errorf("did not terminate after %d intents (turn %d)", n, m.g.Board().Turn))
		}
		// Park the decision if this is the first live iteration, or the
		// previous iteration's end-of-loop park produced nothing because the
		// game just ended (the Over check above would already have caught
		// that; a non-over nil here is the old "engine stalled" crash). The
		// projection runs under the exclusive lock so a focus subscriber
		// cannot project the live engine concurrently; parkSeat — which may
		// call a blocking Decide — runs without holding m.mu.
		if parked == nil {
			var data *parkedData
			err := m.locked(func() error {
				data = projectNext(m, seats, &brd)
				return nil
			})
			if err != nil {
				return r.crash(t, m, err)
			}
			if data == nil {
				return r.crash(t, m, fmt.Errorf("engine stalled: game not over and no decision pending"))
			}
			parked = parkSeat(ctx, seats, data, m.undo.signal, gate, t.stop)
			// onMatchStart's frames went out before this park and no Submit has
			// run yet, so nothing else tells a waiting client about this decision.
			r.fanout(t, m, len(m.g.Log().Events))
		}
		// Await the answer to the parked decision (parked at the first live
		// iteration or at the end of the previous one). A bot seat resolved
		// synchronously in parkSeat and returns immediately; a human seat
		// parks here on its slot until a SubmitIntent arrives or ctx/timeout
		// fires the caretaker. The loop re-parks a human whose answer the
		// engine refused (D1b): refusal preserves the pending decision, so the
		// same seat answers it again instead of the match crashing.
		var in decision.Intent
		var next *parkedDecision
		var nextData *parkedData
		for {
			var err error
			in, err = parked.answer()
			if err != nil {
				// The parked human's await saw the undo signal before any intent:
				// rewind exactly as the top-of-loop check would. The await's own
				// defer has already cleared the seat's slot.
				if req, isUndo := asUndo(err); isUndo {
					var uerr error
					parked, uerr = r.serviceUndo(ctx, t, m, seats, &brd, req)
					if uerr != nil {
						return r.crash(t, m, uerr)
					}
					n, lastTurn, decisionsThisTurn = m.intents-1, m.g.Board().Turn, 0
					continue playLoop
				}
				// BP-10 (spec §7): a SEARCH decision (one that parked on the
				// registry's FIFO search-slot gate) whose seat error surfaced while
				// the table's context is cancelled — its slot wait unblocked by
				// Close, or the search seat unwound by the same cancellation —
				// aborts the match instead of crashing it: a table being closed is
				// not a crash. Every other seat keeps the historical crash contract
				// (Ruling FL-17): a cancelled plain seat crashes the match.
				if parked.searchSlot && (ctx.Err() != nil || errors.Is(err, errTableClosed)) {
					return r.abort(m)
				}
				return r.crash(t, m, fmt.Errorf("seat %d: %w", parked.p, err))
			}
			// HD-2's provenance bits for the intent the seat just gave (the doc
			// on parkedDecision.provenance). They mark THIS seat's own answer;
			// the refusal ladder's rungs below are bots.RefusalAnswerer
			// fallbacks — a HumanSeat is never one, so a human's caretaker/auto
			// bits can never ride a rung, and for a bot seat both bits are false
			// — so the same flags append on every rung the ladder submits.
			caretaker, auto := parked.provenance()
			// attempt runs one Submit attempt as its own exclusive section and
			// reports whether the ENGINE refused it (a submission error) as
			// opposed to a post-Submit failure (persistence). A refused Submit
			// leaves the pending decision intact -- refusal is preserve-and-reject
			// -- so attempt may be called again for the ladder's next rung; a
			// post-Submit failure is fatal and must crash, never retry.
			attempt := func(in decision.Intent) (refused bool, err error) {
				// A panic anywhere inside the attempt (Submit, persistence or
				// the post-Submit projection) unwinds to play's recover, which
				// crashes the match. Reply to the waiting submit first: await
				// already marked it received, so its own defer will not close
				// abandoned, and without this reply the handler would wait
				// forever on a goroutine that is gone.
				defer func() {
					if p := recover(); p != nil {
						parked.reply(fmt.Errorf("match crashed: %v", p))
						panic(p)
					}
				}()
				err = m.locked(func() error {
					before = len(m.g.Log().Events)
					// d is the decision this intent answers. It is read before the
					// Submit because a successful Submit consumes it; on a refusal
					// the pending decision survives and the ladder's next rung
					// answers the same d. A successful Submit also RETIRES the
					// arena generation d lives in before it returns
					// (decision_arena_live.go), so the feed's post-Submit record
					// needs an owned copy taken here, while d is still valid.
					d := m.g.Pending()
					var drec *decision.Decision
					if d != nil && m.feeds != nil {
						// A payment-plan answer (in.Payment / in.Announce) names
						// one of the decision's PaymentActions, which the engine
						// builds lazily -- Submit builds them to validate it. A
						// seat that opts out of PaymentPlanConsumer (az-redeal's
						// manual config, or a test seat) leaves them
						// unbuilt on the live decision, so the copy recorded on
						// the feed could not translate the answer and the feed
						// stopped for the rest of the game. Build them first: a
						// pure derived read, no event, no RNG.
						if in.Payment != nil || in.Announce != nil {
							m.ensurePaymentActions()
						}
						drec = d.Clone()
					}
					if e := m.g.Submit(in); e != nil {
						refused = true
						return e
					}
					m.afterSubmit(before)
					// HD-2: the accepted intent's provenance byte, appended in
					// the same exclusive section (a refused submit appends
					// nothing). server/host/divergence_ledger.go holds the shape.
					m.ledgerAppend(caretaker, auto)
					// Record the ACCEPTED intent (which may be a refusal-ladder
					// rung, not the seat's first answer) on the actor's feed, after
					// the Submit that took it (§5.2 Record).
					m.feeds.record(drec, in)
					if e := r.afterBurst(t, m, before); e != nil { // Tasks 11, 12
						return fmt.Errorf("persist: %w", e)
					}
					// Still exclusive: project the engine's NEXT decision (nil when the
					// game just ended) so a focus subscriber, which also projects the live
					// engine through projectLive's exclusive m.mu, can never run its own
					// projection concurrently with this one (view.Project/view.ProjectFor
					// write the Derived cache). The old loop got the same serialization
					// because its projection immediately preceded this Submit; this keeps
					// it now that the park happens after the Submit.
					nextData = projectNext(m, seats, &brd)
					return nil
				})
				return refused, err
			}
			refused, err := attempt(in)
			if !refused {
				// The engine accepted the seat's answer: release a registry
				// submit waiting for the verdict BEFORE any post-Submit failure
				// can crash the match (the submit must never be stranded on a
				// goroutine that is gone).
				parked.reply(nil)
				if err != nil {
					return r.crash(t, m, err)
				}
				break
			}
			// The engine refused the seat's answer. Only a waiting registry
			// submit has a human to ask again: re-park the same pending
			// decision and tell that submit why its answer was refused (the
			// 409). The old parking's slot was claimed by that submit; the new
			// parking installs a fresh slot so the next POST can answer it.
			// A caretaker or plain Decide answer (ack nil) has no waiting
			// human -- re-parking it would make the same caretaker answer the
			// same refused decision forever -- so it falls through to the
			// ordinary ladder/crash below.
			//
			// There is deliberately no refusal cap: each refusal is one
			// client-initiated POST answered with a 409, and the decision stays
			// pending for a legal answer. A client that never sends one is no
			// worse than a client that never answers at all (the configured
			// ThinkTimeout caretaker still applies), and crashing the whole
			// match on the 17th bad POST would let one seat kill the table.
			if parked.waitingHumanSubmit() {
				m.mu.Lock()
				m.refusals++
				m.mu.Unlock()
				refusedParking := parked
				parked = parkSeat(ctx, seats, refusedParking.data, m.undo.signal, gate, t.stop)
				refusedParking.reply(err)
				continue
			}
			// BP-06 refusal ladder (spec §5.5). Only a seat that opts in with
			// bots.RefusalAnswerer gets the ladder; every other seat keeps
			// today's crash, unchanged.
			ra, ok := seats[parked.p].(bots.RefusalAnswerer)
			if !ok {
				return r.crash(t, m, fmt.Errorf("intent %d rejected: %w", n, err))
			}
			m.mu.Lock()
			m.refusals++
			// Re-project the refused decision's View under the lock, exactly as
			// projectNext would: AnswerRefused runs OUTSIDE it, after this block.
			// The pending decision is where the seat answered, so its owner is
			// parked.p's decision. board fills the reusable per-match botpolicy
			// board the fallback rungs read; it is built under the same lock and
			// consumed before the next decision refills it.
			var dc decision.Decision
			var v view.View
			if d := m.g.Pending(); d != nil {
				dc = *d.Clone()
				v = view.Project(m.g.Board(), m.g.Chars(), dc.Player, &dc)
				v.Round = view.RoundOf(m.g.Board(), m.g.Log().Events)
				botpolicy.BoardFromGameInto(m.g.Board(), m.g.Chars(), dc.Player, &brd)
			}
			m.mu.Unlock()
			// Rung 1: the seat's retry, outside the lock.
			retry := ra.AnswerRefused(v, dc, in)
			refused, err = attempt(retry)
			if err != nil && !refused {
				return r.crash(t, m, err)
			}
			if refused {
				// Rung 2: the shared fallbacks, in order, each its own
				// attempt. The first rung to submit wins. Their intents carry
				// dc's Seq/Player, so each is Submit-ready.
				m.mu.Lock()
				m.fallbacks++
				m.mu.Unlock()
				lastErr := err
				for _, fb := range bots.Fallbacks(&dc, brd, int(parked.p)) {
					refused, err = attempt(fb)
					if err != nil && !refused {
						return r.crash(t, m, err)
					}
					if !refused {
						break
					}
					lastErr = err
				}
				if refused {
					// Rung 3: every rung refused, crash with the last error, as today.
					return r.crash(t, m, fmt.Errorf("intent %d rejected: %w", n, lastErr))
				}
			}
			if err != nil {
				return r.crash(t, m, err)
			}
			break
		}
		// Task HW1: advance the per-turn progress counter now that the
		// Submit's burst is applied. G.Turn only ever increases, so a turn
		// greater than lastTurn means this very decision carried the game
		// into a new turn — reset the count. Otherwise count the decision
		// and, once the whole turn's unanswered decisions reach the limit,
		// crash with a reason naming the stall, the count and the turn (the
		// monospace crash report and the table_halted frame carry it, so an
		// operator sees what happened rather than a generic failure). Read
		// without the lock like the top-of-loop Over check: the loop is the
		// only writer, and readers holding RLock see a stable boundary
		// either side of this section.
		if perTurnLimit > 0 {
			if m.g.Board().Turn > lastTurn {
				lastTurn = m.g.Board().Turn
				decisionsThisTurn = 0
			} else {
				decisionsThisTurn++
				if decisionsThisTurn >= perTurnLimit {
					return r.crash(t, m, fmt.Errorf("stalled: %d decisions answered with no turn advance in turn %d (limit %d)", decisionsThisTurn, m.g.Board().Turn, perTurnLimit))
				}
			}
		}
		// Install the next decision's answerable slot OUTSIDE the match lock:
		// parkSeat may call a blocking Decide (an embedder's seat), and the
		// match mutex must never be held across one. Publishing happens only
		// after this, so the park-before-publish ordering still holds.
		if nextData != nil {
			next = parkSeat(ctx, seats, nextData, m.undo.signal, gate, t.stop)
		}
		// Park the engine's NEXT decision BEFORE publishing it: the seat that
		// owns it is now accept-ready, so the fan-out below cannot expose a
		// decision no seat is waiting to answer — the race defect B fixes.
		// When the game just ended, next stays nil and the top-of-loop Over
		// check finishes. This ordering, on the match goroutine, holds for
		// every consumer of the published state.
		parked = next
		r.fanout(t, m, before) // Task 10
		r.opts.Sleep(t.cfg.Pace, t.stop)
	}
}

// settleNaturalEndOrUndo is the linearization point between an accepted undo
// and a natural match finish. Registry.Undo reserves its request while holding
// m.mu too. Therefore a request that wins the lock is consumed here before the
// G.Over transition, while a finish that wins records MatchFinished before
// Undo can return and causes that request to be rejected. An HTTP 204 can never
// be followed by match_end without the corresponding rewind being serviced.
//
// The test-only barrier runs before this lock only when the game is over; it
// lets the race regression deterministically pause finish while Undo queues.
// The match goroutine is the sole engine writer, so the preliminary G.Over read
// is stable until this function either services an undo or records the finish.
func (r *Registry) settleNaturalEndOrUndo(t *table, m *match) (state.PlayerID, bool, string) {
	if m.g.Board().Over && r.opts.beforeFinish != nil {
		r.opts.beforeFinish()
	}

	m.mu.Lock()
	select {
	case req := <-m.undo.signal:
		m.mu.Unlock()
		return req, true, ""
	default:
	}
	if !m.g.Board().Over {
		m.mu.Unlock()
		return 0, false, ""
	}
	m.state = protocol.MatchFinished
	m.head = m.g.Log().Head()
	if m.g.Board().Draw {
		m.result = "draw"
	} else {
		m.result = "win"
		w := uint8(m.g.Board().Winner)
		m.winner = &w
	}
	m.mu.Unlock()

	r.onMatchEnd(t, m)      // Tasks 10, 12
	r.observeMatchEnd(t, m) // Task M2c-1
	return 0, false, protocol.MatchFinished
}

// abort records a match cut short by Close.
func (r *Registry) abort(m *match) string {
	m.mu.Lock()
	m.state = protocol.MatchAborted
	m.head = m.g.Log().Head()
	m.mu.Unlock()
	r.onMatchEnd(m.table, m)
	r.observeMatchEnd(m.table, m) // Task M2c-1
	return protocol.MatchAborted
}

// crash is spec D15's first half: the match is marked crashed with its
// reason. Task 13 adds the crash report, the table halt frame and tests.
// Fix round 1 (burst atomicity): the head recorded here is over the
// persisted prefix, not the in-memory log — the in-memory log can be ahead
// of disk because the crashed Submit's events never fully reached the
// files, and a sidecar naming a head/count that was never written would
// make a restart serve a log that cannot replay. m.persisted is the last
// fully-appended boundary.
func (r *Registry) crash(t *table, m *match, err error) string {
	m.mu.Lock()
	m.state = protocol.MatchCrashed
	m.reason = err.Error()
	if m.files != nil {
		m.head = m.g.Log().HeadAt(m.persisted)
	} else {
		m.head = m.g.Log().Head()
	}
	m.mu.Unlock()
	r.writeCrashReport(t, m, err.Error())
	r.onMatchEnd(t, m)
	r.observeMatchEnd(t, m) // Task M2c-1
	return protocol.MatchCrashed
}

// matchIdentity derives the stable per-match game id and v2 secret from the
// table seed and match index. The host has no protocol-level game id of its
// own, and an external seat must see the same value for a replayed match.
func matchIdentity(seed uint64, k int) (gameID string, secret []byte) {
	sum := sha256.Sum256([]byte(fmt.Sprintf("gorge-match:%d:%d", seed, k)))
	gameID = fmt.Sprintf("g-%016x", sum[:8])
	return gameID, sum[:]
}

func tableFormatName(f game.Format) string {
	if f == game.FormatCommander {
		return "commander"
	}
	return "constructed"
}

func mulliganName(n int) string {
	if n > 0 {
		return "london"
	}
	return "none"
}
