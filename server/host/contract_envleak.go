package host

// RunEnvLeakContract, the seat-level hidden-card leak row every hosted Env
// policy owes (spec 2026-09-28-hosted-bot-packages §5.3), and the drive
// helpers it shares with the Env tests (botenv_test.go): projectNext under
// the match lock, parkSeat outside it, and the swap fixture.

import (
	"context"
	"reflect"
	"slices"
	"weak"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// envLeakSeed is the seed every leak row (and its pre-pass) drives: one fixed
// seed is what makes the pre-pass's found boundary valid for the row's
// matches — the drive is deterministic from it.
const envLeakSeed = uint64(20260928)

// envRecord is what one DecideEnv call was handed and answered.
type envRecord struct {
	seq     uint64
	rootRef string // env.RootRefused (empty when the root built)
	// engineID and gID are the root engine's and its game's identities
	// (zero when the root was refused). A weak pointer compares equal only
	// to one made from the same live object, even after that object is
	// reclaimed, so identity checks (never the live engine, never reused)
	// hold without keeping the root alive.
	engineID  weak.Pointer[rules.Engine]
	gID       weak.Pointer[state.Game]
	engine    *rules.Engine // env.Search.Engine, only under retainRoots (envSpySeat/envViewSpySeat always set it)
	g         *state.Game   // the root's own game; never the live engine's (same retention rule)
	feed      *searchseat.Feed
	viewRound int32    // env.View.Round
	actorHand []string // the ROOT's hand for the deciding seat (slot 0)
	oppHand   []string // the ROOT's hand for the opponent (slot 1)
	actorLib  []string // the ROOT's library for slot 0, in order
	oppLib    []string // the ROOT's library for slot 1, in order
	in        decision.Intent
}

// zoneNames names the objects of one zone, in zone order. Face-down or
// face-less objects contribute nothing rather than panicking.
func zoneNames(ids []state.ObjID, e *rules.Engine) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if o := e.G.Obj(id); o != nil && o.Card != nil && len(o.Card.Faces) > 0 && o.Card.Faces[0] != nil {
			out = append(out, o.Card.Faces[0].Name)
		}
	}
	return out
}

// sortedEqual compares two name lists as multisets.
func sortedEqual(a, b []string) bool {
	x, y := slices.Clone(a), slices.Clone(b)
	slices.Sort(x)
	slices.Sort(y)
	return slices.Equal(x, y)
}

// hiddenWorldsEqual reports whether two engines hold the same hidden card
// arrangement: hand name MULTISETS and library name ORDERS per player. It is
// the comparison both uses below are built on: the leak test's roots must
// satisfy it name-for-name, and the swap fixture's two live engines must NOT.
func hiddenWorldsEqual(a, b *rules.Engine) bool {
	for pl := state.PlayerID(0); int(pl) < len(a.G.Players); pl++ {
		if !sortedEqual(zoneNames(a.G.Zone(state.ZHand, pl), a), zoneNames(b.G.Zone(state.ZHand, pl), b)) {
			return false
		}
		if !slices.Equal(zoneNames(a.G.Zone(state.ZLibrary, pl), a), zoneNames(b.G.Zone(state.ZLibrary, pl), b)) {
			return false
		}
	}
	return true
}

// envTestTable is the 2-seat table every Env test seats: a spy EnvSeat at
// slot 0, a plain bot at slot 1, fixed seed and fixed repo decks.
func envTestTable(id TableID, seed uint64) TableConfig {
	return TableConfig{ID: id, Name: "env" + string(id), Seats: 2, Decks: []string{"a", "b"},
		Seed: seed, Pace: 0, Spectator: view.Public}
}

// driveEnvStep advances one decision through the REAL host path — projectNext
// under the match's exclusive lock (which observes the feeds and builds the
// Env), parkSeat outside it. beforeProject, when non-nil, runs under the lock
// right before projectNext whenever the pending decision is the EnvSeat's
// (slot 0): the swap fixture and the feed-stop use it to act at a boundary the
// host path is about to read.
func driveEnvStep(t ContractT, m *match, seats []seat.Seat, brd *botpolicy.Board,
	beforeProject func(m *match, d *decision.Decision)) (*parkedData, *parkedDecision) {
	t.Helper()
	m.mu.Lock()
	if beforeProject != nil {
		if d := m.e.Pending(); d != nil && d.Player == 0 {
			beforeProject(m, d)
		}
	}
	data := projectNext(m, seats, brd)
	m.mu.Unlock()
	if data == nil {
		return nil, nil
	}
	pd := parkSeat(context.Background(), seats, data, m.undo.signal, nil, nil)
	if pd.err != nil {
		t.Fatalf("seat %d: DecideEnv/Decide error: %v", pd.p, pd.err)
	}
	return data, pd
}

// submitEnvStep submits one parked answer through the same exclusive section
// play's attempt uses (minus persistence and the refusal ladder: the spy plays
// the wrapped bot's own answer, which the engine accepts).
func submitEnvStep(t ContractT, r *Registry, tbl *table, m *match, pd *parkedDecision) {
	t.Helper()
	err := m.locked(func() error {
		before := len(m.e.L.Events)
		d := m.e.Pending()
		// A posed decision is valid only until the next Submit on the engine
		// that posed it (decision_arena_live.go); the feed is recorded after
		// the Submit, so it needs an owned copy taken here, exactly as the
		// production play loop's feed does (server/host/match.go).
		var drec *decision.Decision
		if d != nil {
			drec = d.Clone()
		}
		if e := m.e.Submit(pd.in); e != nil {
			return e
		}
		m.afterSubmit(before)
		m.feeds.record(drec, pd.in)
		return r.afterBurst(tbl, m, before)
	})
	if err != nil {
		t.Fatalf("submit at seq %d: %v", pd.in.Seq, err)
	}
}

// swapOpponentHidden is the swap fixture of TestRedealWorldsIgnoreTheRealHiddenCards
// (bots/azmcts/redeal_test.go), applied to a LIVE host engine: every
// unpinned player-1 hand card is exchanged with a same-named-DIFFERENT library
// card, then player 1's library is reversed — all through SECRET events, so
// the public projection the actor's feed captures is byte-identical while the
// hidden arrangement is different. It returns the number of hand cards
// swapped; zero means nothing could be swapped (a fixture failure the caller
// must report).
func swapOpponentHidden(t ContractT, e *rules.Engine, feed *searchseat.Feed) int {
	t.Helper()
	pinned := map[string]bool{}
	if feed != nil {
		known, err := feed.Known()
		if err != nil {
			t.Fatalf("known projection: %v", err)
		}
		for _, kh := range known.Hands {
			if kh.Player == 1 {
				for _, c := range kh.Cards {
					pinned[c.Name] = true
				}
			}
		}
	}
	hand, lib := e.G.Zone(state.ZHand, 1), e.G.Zone(state.ZLibrary, 1)
	swapped := 0
	used := map[state.ObjID]bool{}
	for _, h := range append([]state.ObjID(nil), hand...) {
		hn := e.G.Obj(h).Card.Faces[0].Name
		if pinned[hn] {
			continue
		}
		for _, l := range lib {
			if used[l] || e.G.Obj(l).Card.Faces[0].Name == hn {
				continue
			}
			used[l] = true
			events.Emit(e.G, e.L, events.Event{Kind: events.MoveZone, Player: 1, Obj: h, From: state.ZHand, To: state.ZLibrary, Secret: true})
			events.Emit(e.G, e.L, events.Event{Kind: events.MoveZone, Player: 1, Obj: l, From: state.ZLibrary, To: state.ZHand, Secret: true})
			swapped++
			break
		}
	}
	if swapped == 0 {
		return 0
	}
	cur := e.G.Zone(state.ZLibrary, 1)
	rev := make([]state.ObjID, len(cur))
	for i, id := range cur {
		rev[len(cur)-1-i] = id
	}
	events.Emit(e.G, e.L, events.Event{Kind: events.LibraryOrder, Player: 1, IDs: rev, Secret: true})
	return swapped
}

// EnvLeakContract configures RunEnvLeakContract for one hosted Env policy.
type EnvLeakContract struct {
	// Options builds the registries the row drives (a deck loader serving
	// "a" and "b" at least). The row overrides nothing in it.
	Options Options
	// Policy names the policy in messages.
	Policy string
	// NewSeat builds a FRESH seat of the policy for one match (a hosted bot
	// carries its own RNG state, so the two driven matches never share one).
	NewSeat func(actor uint64) seat.Seat
	// ViewPath selects the View-path spy, for a policy that implements no
	// heuristic.BoardSeat (sb-tactical and the SpellBench adapters): a spy
	// with a DecideBoard would send the host down a branch the policy never
	// takes in production. It forwards seat.PaymentPlanConsumer and
	// bots.RefusalAnswerer.
	ViewPath bool
	// Probe, when non-nil, places the swap at the first decision the seat
	// actually SEARCHED (found by a deterministic unswapped pre-pass of the
	// same seed and seats) and re-verifies that both matches searched it:
	// a search that skips a decision without consulting its world source
	// would otherwise make the row vacuous.
	Probe *SearchProbe
	// PlannerOf, when non-nil, returns the planner the policy's seat (the
	// one NewSeat built) holds after answering; at the asserted boundary it
	// must be the honest root the Env carried, in both matches (the §5.1
	// SetPlanner routing). It returns the planner as an any so this
	// signature names no engine type.
	PlannerOf func(seat.Seat) any
}

// SearchProbe reads a policy's own search-diagnostic stream (azmcts.Watch,
// sbsearch.Watch, ...), which the caller installs for the row's duration.
type SearchProbe struct {
	// Mark returns the number of diagnostics observed so far.
	Mark func() int
	// Searched reports whether diagnostics [from, to) record a decision the
	// seat really searched.
	Searched func(from, to int) bool
}

// RunEnvLeakContract is the hidden-card leak row: two live matches built
// around the REAL hosted policy at slot 0 (a heuristic bot at slot 1), driven
// through the REAL host Env path, with the swap fixture (swapOpponentHidden)
// applied to the second match's live engine at the first decision the policy
// wants an Env for (or, with a Probe, the first decision it searched). The
// decision played at the asserted boundary must be answered identically from
// two honest roots whose hidden worlds are name-identical, while the two live
// positions are not: what the honest root deals is a function of the seat's
// observation and the seed alone. It returns the asserted decision's seq.
func RunEnvLeakContract(t ContractT, c EnvLeakContract) uint64 {
	t.Helper()
	spyFor := func(s seat.Seat) envSpy { return &envSpySeat{inner: s} }
	if c.ViewPath {
		spyFor = func(s seat.Seat) envSpy { return &envViewSpySeat{inner: s} }
	}
	var probe *leakProbe
	var swapSeq uint64
	searched := [3]bool{} // indexed by match 1|2
	if c.Probe != nil || c.PlannerOf != nil {
		probe = &leakProbe{target: func(spy1, spy2 envSpy, boundary uint64) uint64 {
			r1, r2 := spy1.recorded(boundary), spy2.recorded(boundary)
			if r1 == nil || r2 == nil || r1.engine == nil || r2.engine == nil {
				return 0
			}
			if c.PlannerOf != nil {
				for i, rec := range []*envRecord{r1, r2} {
					sp := []envSpy{spy1, spy2}[i]
					if got := c.PlannerOf(sp.innerSeat()); got != any(rec.engine) {
						t.Fatalf("%s slot %d: the adapter's planner is not the Env's honest root: the SetPlanner routing did not run at the asserted boundary", c.Policy, i)
					}
				}
			}
			return boundary
		}}
	}
	if c.Probe != nil {
		swapSeq = firstSearched(t, c, spyFor)
		probe.diagCount = c.Probe.Mark
		probe.onDecided = func(m int, seq uint64, from, to int) {
			if seq == swapSeq && c.Probe.Searched(from, to) {
				searched[m] = true
			}
		}
		probe.swapAt = func(d *decision.Decision) bool { return d.Seq == swapSeq }
	}
	asserted := hostedEnvLeakRow(t, c, probe, spyFor)
	if c.Probe != nil {
		// The pre-pass picked the boundary under the same seed and seats and
		// the drive is deterministic, but that is a claim, not a proof: a
		// regression that stops the search (the adapter no longer forwarding
		// env.Search) would otherwise pass the row silently.
		if !searched[1] || !searched[2] {
			t.Fatalf("%s: the asserted boundary (seq %d) was not SEARCHED in both matches (searched %v/%v): the adapter's env.Search forwarding is not exercised and this row would be vacuous",
				c.Policy, swapSeq, searched[1], searched[2])
		}
		t.Logf("%s: asserted boundary seq %d was searched in both matches", c.Policy, swapSeq)
	}
	return asserted
}

// firstSearched is the probe row's pre-pass: one unswapped match of the
// policy, driven through the REAL host Env path from the row's seed and the
// row's seats (the spy wrapper delegates identically and consumes the same
// RNG stream), until the seat's own search actually runs once. It returns
// that decision's seq.
func firstSearched(t ContractT, c EnvLeakContract, spyFor func(seat.Seat) envSpy) uint64 {
	t.Helper()
	r, err := New(c.Options)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()
	if err := r.AddTable(envTestTable("t1", envLeakSeed)); err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	r.mu.RLock()
	tbl := r.tables["t1"]
	r.mu.RUnlock()
	seats := []seat.Seat{spyFor(c.NewSeat(envLeakSeed ^ 1)), heuristic.NewBot(envLeakSeed ^ 1)}
	m, err := r.newMatch(tbl, 0)
	if err != nil {
		t.Fatalf("newMatch: %v", err)
	}
	m.mu.Lock()
	m.slots = seats
	m.feeds = newMatchFeeds(seats)
	m.mu.Unlock()
	brd := botpolicy.NewBoard(2)
	for steps := 0; steps < 20000; steps++ {
		n0 := c.Probe.Mark()
		_, pd := driveEnvStep(t, m, seats, &brd, nil)
		if pd == nil {
			t.Fatalf("%s pre-pass: the match ended without the seat ever searching a decision", c.Policy)
		}
		n1 := c.Probe.Mark()
		submitEnvStep(t, r, tbl, m, pd)
		if c.Probe.Searched(n0, n1) {
			return pd.in.Seq
		}
	}
	t.Fatalf("%s pre-pass: the step cap was reached without the seat ever searching a decision", c.Policy)
	return 0
}

// envSpySeat wraps a REAL hosted EnvSeat and records exactly what the host
// handed each DecideEnv call, so a leak row can assert on the honest root the
// real adapter was handed while the adapter itself answers. Every half
// delegates to the wrapped seat; the wrapper implements bots.EnvSeat itself,
// so the host builds a feed and Envs for it exactly as for the bare adapter.
type envSpySeat struct {
	inner seat.Seat // a bots.EnvSeat at runtime; the built hosted policy
	envs  []envRecord
}

func (s *envSpySeat) wrapped() bots.EnvSeat {
	es, ok := s.inner.(bots.EnvSeat)
	if !ok {
		panic("envSpySeat wraps a non-EnvSeat policy")
	}
	return es
}

func (s *envSpySeat) WantsEnv(d *decision.Decision) bool { return s.wrapped().WantsEnv(d) }

func (s *envSpySeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	rec := recordEnv(env, d)
	in, err := s.wrapped().DecideEnv(ctx, env, d)
	rec.in = in
	s.envs = append(s.envs, rec)
	return in, err
}

func (s *envSpySeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.inner.Decide(ctx, v, d)
}

func (s *envSpySeat) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	bs, ok := s.inner.(heuristic.BoardSeat)
	if !ok {
		// Every Board-path hosted policy is a BoardSeat; a non-BoardSeat
		// would be parked by the host on a zero View, a wiring bug.
		panic("envSpySeat wraps a non-BoardSeat policy")
	}
	return bs.DecideBoard(ctx, b, d)
}

func (s *envSpySeat) recorded(seq uint64) *envRecord { return recordAt(s.envs, seq) }
func (s *envSpySeat) envCount() int                  { return len(s.envs) }
func (s *envSpySeat) innerSeat() seat.Seat           { return s.inner }

// envViewSpySeat is the View-path spy of the same recording contract: it
// wraps a hosted EnvSeat that does NOT implement heuristic.BoardSeat, so it
// deliberately carries no DecideBoard. The optional interfaces a hosted bot
// may carry — seat.PaymentPlanConsumer and bots.RefusalAnswerer — are
// forwarded, so the host treats the wrapper exactly as the bare adapter.
type envViewSpySeat struct {
	inner seat.Seat // the built hosted policy
	envs  []envRecord
}

func (s *envViewSpySeat) innerSeat() seat.Seat { return s.inner }

func (s *envViewSpySeat) WantsEnv(d *decision.Decision) bool {
	return s.inner.(bots.EnvSeat).WantsEnv(d)
}

func (s *envViewSpySeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	rec := recordEnv(env, d)
	in, err := s.inner.(bots.EnvSeat).DecideEnv(ctx, env, d)
	rec.in = in
	s.envs = append(s.envs, rec)
	return in, err
}

func (s *envViewSpySeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	return s.inner.Decide(ctx, v, d)
}

func (s *envViewSpySeat) WantsPaymentActions() bool {
	if pc, ok := s.inner.(seat.PaymentPlanConsumer); ok {
		return pc.WantsPaymentActions()
	}
	return false
}

func (s *envViewSpySeat) AnswerRefused(v view.View, d decision.Decision, refused decision.Intent) decision.Intent {
	return s.inner.(bots.RefusalAnswerer).AnswerRefused(v, d, refused)
}

func (s *envViewSpySeat) recorded(seq uint64) *envRecord { return recordAt(s.envs, seq) }
func (s *envViewSpySeat) envCount() int                  { return len(s.envs) }

// recordEnv is the record both spies take of one Env: the root itself is
// retained (the row compares two roots' hidden worlds after the fact).
func recordEnv(env bots.Env, d decision.Decision) envRecord {
	rec := envRecord{seq: d.Seq, rootRef: env.RootRefused}
	if e := env.Search.Engine; e != nil {
		rec.engine, rec.g = e, e.G
		rec.actorHand = zoneNames(e.G.Zone(state.ZHand, 0), e)
		rec.oppHand = zoneNames(e.G.Zone(state.ZHand, 1), e)
		rec.actorLib = zoneNames(e.G.Zone(state.ZLibrary, 0), e)
		rec.oppLib = zoneNames(e.G.Zone(state.ZLibrary, 1), e)
	}
	return rec
}

func recordAt(envs []envRecord, seq uint64) *envRecord {
	for i := range envs {
		if envs[i].seq == seq {
			return &envs[i]
		}
	}
	return nil
}

// envSpy is the recording-spy interface the row drives: one wrapper per
// match.
type envSpy interface {
	seat.Seat
	recorded(seq uint64) *envRecord
	envCount() int
	innerSeat() seat.Seat
}

// leakProbe is the row's optional extension point: it attributes the
// policy's own search diagnostics to the parked decisions and moves the
// asserted boundary off the swap boundary.
type leakProbe struct {
	// diagCount returns the number of diagnostics observed so far; the
	// helper reads it immediately before and after each parkSeat. With the
	// two matches driven synchronously one step at a time, the diagnostics
	// that appeared during match m's park belong to that match's decision.
	diagCount func() int
	// onDecided is called after each parked decision of match m (1 or 2) at
	// seq with the diagnostic range [from, to) its park produced.
	onDecided func(m int, seq uint64, from, to int)
	// swapAt, when non-nil, replaces the swap gate (the first Env-eligible
	// decision).
	swapAt func(d *decision.Decision) bool
	// target, when non-nil, is called after each full drive round once the
	// swap is in place; it returns the seq to assert on (>0) or 0 to keep
	// driving.
	target func(spy1, spy2 envSpy, swapSeq uint64) uint64
}

func probeDiags(p *leakProbe) int {
	if p == nil || p.diagCount == nil {
		return 0
	}
	return p.diagCount()
}

// hostedEnvLeakRow is the row's body (see RunEnvLeakContract).
func hostedEnvLeakRow(t ContractT, c EnvLeakContract, probe *leakProbe, spyFor func(seat.Seat) envSpy) uint64 {
	t.Helper()
	const seed = envLeakSeed
	r, err := New(c.Options)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	defer r.Close()
	if err := r.AddTable(envTestTable("t1", seed)); err != nil {
		t.Fatalf("AddTable: %v", err)
	}
	r.mu.RLock()
	tbl := r.tables["t1"]
	r.mu.RUnlock()

	build := func(actor uint64) (*match, envSpy, []seat.Seat) {
		spy := spyFor(c.NewSeat(actor))
		seats := []seat.Seat{spy, heuristic.NewBot(actor)}
		m, err := r.newMatch(tbl, 0)
		if err != nil {
			t.Fatalf("newMatch: %v", err)
		}
		m.mu.Lock()
		m.slots = seats
		m.feeds = newMatchFeeds(seats)
		m.mu.Unlock()
		return m, spy, seats
	}
	m1, spy1, seats1 := build(seed ^ 1)
	m2, spy2, seats2 := build(seed ^ 1)

	var swapped bool
	var targetSeq uint64
	swapFn := func(m *match, d *decision.Decision) {
		if swapped {
			return
		}
		if probe != nil && probe.swapAt != nil {
			if !probe.swapAt(d) {
				return
			}
		} else if !seats2[0].(bots.EnvSeat).WantsEnv(d) {
			return
		}
		if n := swapOpponentHidden(t, m.e, m.feeds.bySlot[0]); n > 0 {
			swapped = true
			targetSeq = d.Seq
		}
	}

	brd1, brd2 := botpolicy.NewBoard(2), botpolicy.NewBoard(2)
	assertSeq := uint64(0)
	for steps := 0; steps < 20000; steps++ {
		before1 := probeDiags(probe)
		_, pd1 := driveEnvStep(t, m1, seats1, &brd1, nil)
		after1 := probeDiags(probe)
		if pd1 == nil {
			t.Fatalf("%s: match 1 ended before the asserted boundary was reached", c.Policy)
		}
		if probe != nil && probe.onDecided != nil {
			probe.onDecided(1, pd1.in.Seq, before1, after1)
		}
		before2 := probeDiags(probe)
		_, pd2 := driveEnvStep(t, m2, seats2, &brd2, swapFn)
		after2 := probeDiags(probe)
		if pd2 == nil {
			t.Fatalf("%s: match 2 ended before the asserted boundary was reached", c.Policy)
		}
		if probe != nil && probe.onDecided != nil {
			probe.onDecided(2, pd2.in.Seq, before2, after2)
		}
		submitEnvStep(t, r, tbl, m1, pd1)
		submitEnvStep(t, r, tbl, m2, pd2)
		if probe != nil && probe.target != nil && swapped {
			if want := probe.target(spy1, spy2, targetSeq); want != 0 {
				assertSeq = want
				break
			}
			continue
		}
		if rec1, rec2 := spy1.recorded(targetSeq), spy2.recorded(targetSeq); swapped && rec1 != nil && rec2 != nil {
			assertSeq = targetSeq
			break
		}
	}
	if !swapped {
		t.Fatalf("%s fixture: no opponent hand card could be swapped at any decision the policy wants an Env for", c.Policy)
	}
	if assertSeq == 0 && probe != nil && probe.target != nil {
		t.Fatalf("fixture: %s: the asserted boundary (swap seq %d) never reached both DecideEnv records: the two matches diverged before it or the drive hit its cap, and the leak claim cannot be asserted", c.Policy, targetSeq)
	}
	if assertSeq == 0 {
		assertSeq = targetSeq
	}
	t.Logf("%s swap target: seq %d (an Env-eligible decision); asserted boundary: seq %d", c.Policy, targetSeq, assertSeq)
	rec1, rec2 := spy1.recorded(assertSeq), spy2.recorded(assertSeq)
	if rec1 == nil || rec2 == nil {
		t.Fatalf("%s: the asserted decision (seq %d) was never parked on DecideEnv (records %d/%d)",
			c.Policy, assertSeq, spy1.envCount(), spy2.envCount())
	}
	if rec1.engine == nil || rec2.engine == nil {
		t.Fatalf("%s: an honest root was refused at the asserted boundary (%q / %q): the adapter was never handed a world",
			c.Policy, rec1.rootRef, rec2.rootRef)
	}
	// Fixture precondition: the two live positions genuinely differ in
	// hidden cards — the swap did its work.
	if hiddenWorldsEqual(m1.e, m2.e) {
		t.Fatalf("%s fixture: the two live positions hold the same hidden cards", c.Policy)
	}
	// Central claim: the two roots are name-identical in every hidden zone,
	// and the decision the real adapter answered is identical in both
	// worlds. The swap happened BEFORE the asserted boundary, so a root that
	// read the real hidden cards would differ here.
	if !hiddenWorldsEqual(rec1.engine, rec2.engine) {
		t.Fatalf("%s: two roots for the same seed and feed depend on the real hidden cards:\nreal hand %v swapped %v\nreal lib %v swapped %v",
			c.Policy, rec1.oppHand, rec2.oppHand, rec1.oppLib, rec2.oppLib)
	}
	if !reflect.DeepEqual(rec1.in, rec2.in) {
		t.Fatalf("%s: the two matches answered the swapped decision differently: %+v vs %+v", c.Policy, rec1.in, rec2.in)
	}
	// The deciding seat's own zones are untouched by the fixture and must
	// match in the roots too (the redeal keeps what the seat knows).
	if !sortedEqual(rec1.actorHand, rec2.actorHand) {
		t.Fatalf("%s: the two roots dealt the actor's own hand differently", c.Policy)
	}
	return assertSeq
}
