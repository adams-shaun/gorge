package host

// RunShadowParityContract, the shadow-scan parity row
// (docs/design/2026-10-10-human-divergence-design.md §5.3-§5.5) for a hosted
// policy, and the shadow test seat the host's own shadow-scan tests drive.

import (
	"context"
	"fmt"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/bots/heuristic"
	"github.com/adams-shaun/gorge/bots/searchseat"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/engine/seat"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
	"github.com/adams-shaun/gorge/server/divergence"
	"github.com/adams-shaun/gorge/server/protocol"
)

// shadowTestSeat plays slot 0 the way the shadow scan assumes a seat plays:
// at every decision it builds a FRESH bots.New(policy, ShadowSeed(matchSeed,
// slot, n)) where n is the decision's intent index (the intents already
// logged), and answers through the same dispatch order the host uses for it
// (Env, then Board, then View). For an Env decision the honest root is dealt
// from RootSeed(shadowSeed, seq), as shadow.Answer does, because the host
// builds roots from the per-seat seed instead. deviate, when set, may swap
// the shadow-equal answer for another one (the injected-divergence test).
// The seat runs on the match goroutine, so reading the live engine's log is
// race-free.
type shadowTestSeat struct {
	policy    string
	deps      bots.Deps
	matchSeed uint64
	slot      state.PlayerID
	live      func() *rules.Engine
	probe     seat.Seat
	deviate   func(d *decision.Decision, n int, b decision.Intent) (decision.Intent, bool)

	seen    []int // intent index of every decision answered
	envs    int   // decisions answered through the Env path
	multi   int   // decisions that offered more than one option
	refused int   // Env decisions whose honest root was refused
}

func newShadowTestSeat(t ContractT, policy string, deps bots.Deps, matchSeed uint64, live func() *rules.Engine) *shadowTestSeat {
	t.Helper()
	probe, err := bots.New(policy, bots.Options{Seed: 1, Deps: deps})
	if err != nil {
		t.Fatalf("bots.New(%q): %v", policy, err)
	}
	return &shadowTestSeat{policy: policy, deps: deps, matchSeed: matchSeed, live: live, probe: probe}
}

func (s *shadowTestSeat) index() (int, uint64) {
	n := len(s.live().L.Intents)
	return n, divergence.ShadowSeed(s.matchSeed, s.slot, n)
}

// answeredBelow counts the seat's answered decisions whose intent index is
// below logged, the decisions a match log of logged intents records.
func (s *shadowTestSeat) answeredBelow(logged int) int {
	n := 0
	for _, i := range s.seen {
		if i < logged {
			n++
		}
	}
	return n
}

// loggedIntents is match k's recorded intent count.
func (r *Registry) loggedIntents(id TableID, k int) (int, error) {
	_, m, err := r.lookup(id, k)
	if err != nil {
		return 0, fmt.Errorf("host: table %s match %d: %w", id, k, err)
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.e.L.Intents), nil
}

func (s *shadowTestSeat) fresh(seed uint64) seat.Seat {
	in, err := bots.New(s.policy, bots.Options{Seed: seed, Deps: s.deps})
	if err != nil {
		panic(err)
	}
	return in
}

func (s *shadowTestSeat) finish(d decision.Decision, n int, in decision.Intent, err error) (decision.Intent, error) {
	s.seen = append(s.seen, n)
	if len(d.Options) > 1 {
		s.multi++
	}
	if err == nil && s.deviate != nil {
		if alt, ok := s.deviate(&d, n, in); ok {
			return alt, nil
		}
	}
	return in, err
}

func (s *shadowTestSeat) WantsPaymentActions() bool {
	c, ok := s.probe.(seat.PaymentPlanConsumer)
	return ok && c.WantsPaymentActions()
}

func (s *shadowTestSeat) WantsEnv(d *decision.Decision) bool {
	es, ok := s.probe.(bots.EnvSeat)
	return ok && es.WantsEnv(d)
}

func (s *shadowTestSeat) DecideEnv(ctx context.Context, env bots.Env, d decision.Decision) (decision.Intent, error) {
	n, seed := s.index()
	s.envs++
	root, reason := searchseat.HonestRoot(env.Search.Setup, env.Search.Feed, s.live(), bots.RootSeed(seed, d.Seq))
	if root == nil {
		s.refused++
	}
	env.Search.Engine, env.RootRefused = root, reason
	in, err := s.fresh(seed).(bots.EnvSeat).DecideEnv(ctx, env, d)
	return s.finish(d, n, in, err)
}

// shadowBoardSeat adds the BoardSeat face. Only a policy whose own seat is a
// BoardSeat gets it, so the host dispatches the test seat exactly as it
// would dispatch the policy (a type with a DecideBoard method is a
// BoardSeat whether or not the wrapped policy is).
type shadowBoardSeat struct{ *shadowTestSeat }

func (s shadowBoardSeat) DecideBoard(ctx context.Context, b botpolicy.Board, d decision.Decision) (decision.Intent, error) {
	n, seed := s.index()
	in, err := s.fresh(seed).(heuristic.BoardSeat).DecideBoard(ctx, b, d)
	return s.finish(d, n, in, err)
}

func (s *shadowTestSeat) Decide(ctx context.Context, v view.View, d decision.Decision) (decision.Intent, error) {
	n, seed := s.index()
	in, err := s.fresh(seed).Decide(ctx, v, d)
	return s.finish(d, n, in, err)
}

// liveEngineOf returns table id's current match engine (nil before it starts).
func liveEngineOf(r *Registry, id TableID) *rules.Engine {
	r.mu.RLock()
	tb := r.tables[id]
	r.mu.RUnlock()
	if tb == nil {
		return nil
	}
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	if tb.cur == nil {
		return nil
	}
	return tb.cur.e
}

// shadowSeatTable plays a 2-seat bot table whose seat 0 is a shadowTestSeat
// and whose seat 1 is the hosted policy, to completion, and returns the
// registry and the seat. o is the registry's base options (its BotDeps are
// the policy's card dependency); MaxIntents, BotUnboundedDecisions and Seats
// are set over it.
func shadowSeatTable(t ContractT, o Options, policy string, seed uint64, maxIntents int, mk func(*shadowTestSeat)) (*Registry, *shadowTestSeat) {
	t.Helper()
	o.MaxIntents = maxIntents // 0 = play the whole game
	o.BotUnboundedDecisions = true
	var r *Registry
	var ss *shadowTestSeat
	o.Seats = func(names []string, mseed uint64) []seat.Seat {
		ss = newShadowTestSeat(t, policy, o.BotDeps, mseed, func() *rules.Engine { return liveEngineOf(r, "t1") })
		if mk != nil {
			mk(ss)
		}
		opp, err := bots.New(bots.Default, bots.Options{Seed: mseed ^ 2, Deps: o.BotDeps})
		if err != nil {
			t.Fatalf("%v", err)
		}
		if _, ok := ss.probe.(heuristic.BoardSeat); ok {
			return []seat.Seat{shadowBoardSeat{ss}, opp}
		}
		return []seat.Seat{ss, opp}
	}
	r, err := New(o)
	if err != nil {
		t.Fatalf("%v", err)
	}
	t.Cleanup(func() { r.Close() })
	cfg := TableConfig{ID: "t1", Name: "shadow", Seats: 2, Decks: []string{"a", "b"},
		Seed: seed, Pace: 0, Spectator: view.Omniscient, BotPolicy: bots.Default}
	if err := r.AddTable(cfg); err != nil {
		t.Fatalf("%v", err)
	}
	if cfg.autoPayManaEnabled() {
		t.Fatalf("fixture assumes a manual-pay table: the test seat builds its shadow without AutoPayMana")
	}
	if err := r.Start("t1"); err != nil {
		t.Fatalf("%v", err)
	}
	r.Wait("t1")
	ms, err := r.Matches("t1")
	// A capped match ends "crashed" with the intent-cap reason by design
	// (play's MaxIntents guard); its log is a faithful prefix to scan.
	if err != nil || len(ms) != 1 || (maxIntents == 0 && ms[0].State != protocol.MatchFinished) || (maxIntents > 0 && ms[0].State != protocol.MatchFinished && ms[0].State != protocol.MatchCrashed) {
		reason := ""
		if snap, serr := r.SnapshotMatch("t1", 1, nil); serr == nil {
			reason = snap.Match.Reason
		}
		t.Fatalf("match did not finish: %+v %v reason: %s", ms, err, reason)
	}
	return r, ss
}

// RunShadowParityContract is the shadow-scan parity row for policy: a 2-seat
// table whose seat 0 answers every decision with a FRESH bots.New(policy)
// seeded ShadowSeed(seed, 0, n) through the host's own dispatch (Env, then
// Board, then View) is, to the scan, indistinguishable from its shadow:
// ShadowScan reports 0 divergences over every decision the seat answered.
// maxIntents caps the match (0 plays the whole game; a search policy is
// capped, since a searched decision costs a whole search). o is the
// registry's base options; set o.BotDeps to the policy's card dependency.
func RunShadowParityContract(t ContractT, o Options, policy string, seed uint64, maxIntents int) {
	t.Helper()
	r, ss := shadowSeatTable(t, o, policy, seed, maxIntents, nil)
	divs, scanned, err := r.shadowScan("t1", 1, 0, policy)
	if err != nil {
		t.Fatalf("ShadowScan: %v", err)
	}
	// Preconditions: the scan visited exactly the decisions the seat
	// answered, there were enough of them, they were real choices, and (for
	// an Env policy) the Env path really ran. A capped match parks (and the
	// seat answers) the decision at the cap before play's MaxIntents guard
	// ends it, and that answer is never submitted, so only the answers the
	// log records (intent index below len(log.Intents)) count.
	logged, err := r.loggedIntents("t1", 1)
	if err != nil {
		t.Fatalf("%v", err)
	}
	answered := ss.answeredBelow(logged)
	if scanned != answered {
		t.Fatalf("scan visited %d decisions, the seat answered %d the log records", scanned, answered)
	}
	if scanned < 20 || ss.multi < 5 {
		t.Fatalf("degenerate fixture: %d decisions, %d with a choice", scanned, ss.multi)
	}
	if e, _ := bots.Lookup(policy); e.Env && ss.envs == 0 {
		t.Fatalf("policy %q is an EnvSeat but no decision took the Env path", policy)
	}
	t.Logf("%s: %d decisions, %d with a choice, %d Env, %d roots refused", policy, scanned, ss.multi, ss.envs, ss.refused)
	if len(divs) != 0 {
		t.Fatalf("parity broken: %d divergences, first at intent %d (%s: human %q shadow %q)",
			len(divs), divs[0].Intent, divs[0].Material, divs[0].Human.Label, divs[0].Shadow.Label)
	}
}
