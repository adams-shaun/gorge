package host

// Task HW1: the host's per-turn progress guard. A bot policy that keeps
// re-offering one no-op action never lets the turn advance, and before this
// task such a match sat TableLive forever — no match ever followed it —
// because Options.ThinkTimeout covers only a HumanSeat parked on a decision.
// These tests pin the count-based guard that halts a stalled table instead:
// it counts DECISIONS answered since the last turn advance (never a wall
// clock, so the same seed still produces the same chain on every machine),
// resets when the turn advances, and is opt-in — 0 means "no guard".

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// stallCard parses one card script inline (cards.ParseBytes + Link +
// ApplyIntrinsics, the same shape testutil.parseCard uses for its sample
// decks) — no GPL corpus, no file.
func stallCard(t *testing.T, src string) *cards.Card {
	t.Helper()
	c, diags := cards.ParseBytes("stall_test.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse %q: %v", src, diags)
	}
	c.Link()
	for _, f := range c.Faces {
		f.ApplyIntrinsics()
	}
	return c
}

// stallDeck is a 40-card deck of one 0-cost creature with a free, repeatable
// activated ability. That combination guarantees a board-independent,
// engine-side stall the stub seat below can drive forever inside ONE turn:
// the active player casts the creature (a non-pass action — priority stays
// with them and the turn never advances), and once the hand is empty the
// same player can activate the free ability again and again — the same
// "offer and re-offer one legal no-op" signature as the measured equip
// loop, produced here without reproducing it (the brief): no equipment, no
// creatures to equip, just the smallest reusable legal action the engine
// offers.
func stallDeck(t *testing.T) []*cards.Card {
	t.Helper()
	c := stallCard(t, "Name:Keen-Stallion\nTypes:Creature Stallion\nPT:1/1\n"+
		"A:AB$ GainLife | LifeAmount$ 1 | SpellDescription$ gain 1 life\nOracle:x\n")
	out := make([]*cards.Card, 40)
	for i := range out {
		out[i] = c
	}
	return out
}

// stallLoader serves the four stall decks under the fourSeatTable names.
func stallLoader(t *testing.T) func(string) (Deck, error) {
	t.Helper()
	decks := map[string][]*cards.Card{
		"a": stallDeck(t),
		"b": stallDeck(t),
		"c": stallDeck(t),
		"d": stallDeck(t),
	}
	return func(name string) (Deck, error) {
		cs, ok := decks[name]
		if !ok {
			return Deck{}, ErrNotFound
		}
		return Deck{Name: name, Cards: cs}, nil
	}
}

// stallSeat answers every decision with the same no-op intent: option index
// 0 — the first legal action, whatever it is — with the decision's own
// Seq/Player so the engine always accepts it. It is a stub, deliberately NOT
// a copy of any bot policy: it just keeps accepting whatever the engine
// offers, and stallDeck guarantees the offer never runs dry, so the turn
// stays forever on the active seat's priority without ever advancing.
type stallSeat struct{}

func (stallSeat) Decide(_ context.Context, _ view.View, d decision.Decision) (decision.Intent, error) {
	return decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{0}}, nil
}

func fourStallSeats(names []string, seed uint64) []seat.Seat {
	out := make([]seat.Seat, len(names))
	for i := range out {
		out[i] = stallSeat{}
	}
	return out
}

// tableReason is the halt reason recorded on the table (TableHalted's); it
// is the string the table_halted frame and crash report carry, so reading it
// here is exactly what an operator would read.
func tableReason(t *testing.T, r *Registry, id TableID) string {
	t.Helper()
	tb := tableRef(t, r, id)
	tb.mu.RLock()
	defer tb.mu.RUnlock()
	return tb.reason
}

// tableRef resolves a table under the registry lock, the read shape every
// other host test uses (r.tables is guarded by Registry.mu).
func tableRef(t *testing.T, r *Registry, id TableID) *table {
	t.Helper()
	r.mu.RLock()
	tb := r.tables[id]
	r.mu.RUnlock()
	if tb == nil {
		t.Fatalf("no table %s", id)
	}
	return tb
}

// liveIntents reads the live match's answered-decision count — positive
// evidence that the stalled loop is genuinely churning, not parked or ended.
func liveIntents(t *testing.T, r *Registry, id TableID) int {
	t.Helper()
	tb := tableRef(t, r, id)
	tb.mu.RLock()
	m := tb.cur
	if m == nil {
		tb.mu.RUnlock()
		return -1
	}
	m.mu.RLock()
	n := m.intents
	m.mu.RUnlock()
	tb.mu.RUnlock()
	return n
}

// TestATurnThatNeverAdvancesHaltsTheTable is the guard's central contract: a
// bot (or any seat) that keeps answering decisions while the turn never
// advances — the measured equip-loop signature — halts the table, and the
// reason names the stall and the count so an operator reading the frame or
// the crash report knows what happened rather than seeing a generic failure.
func TestATurnThatNeverAdvancesHaltsTheTable(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.LoadDeck = stallLoader(t)
	o.MaxDecisionsPerTurn = 25 // small, so the test trips in milliseconds
	o.Seats = fourStallSeats
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(fourSeatTable("t1", false)); err != nil {
		t.Fatal(err)
	}
	s := r.OpenSession()
	if err := r.Subscribe(s, "t1", protocol.ModeFocus); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	if got := r.Tables()[0].State; got != protocol.TableHalted {
		t.Fatalf("table state %s, want TableHalted", got)
	}
	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchCrashed {
		t.Fatalf("matches %+v", ms)
	}
	reason := tableReason(t, r, "t1")
	// The stall wording, the count that tripped it and the turn it happened
	// in must all be in the recorded reason.
	for _, want := range []string{"stalled", "25", "turn 1"} {
		if !strings.Contains(reason, want) {
			t.Fatalf("halt reason lacks %q: %q", want, reason)
		}
	}
	// The frame the lobby renders carries the same reason.
	var haltReason string
	for _, f := range drainNow(s) {
		if f.T == protocol.TTableHalted {
			var b protocol.TableHaltedBody
			if err := json.Unmarshal(f.Body, &b); err == nil {
				haltReason = b.Reason
			}
		}
	}
	if !strings.Contains(haltReason, "stalled") || !strings.Contains(haltReason, "25") {
		t.Fatalf("table_halted frame reason %q does not name the stall", haltReason)
	}
	// The engine itself never left turn 1: the guard is a host-level trip, it
	// must never have touched the event chain (the match's recorded head is a
	// crash head over the persisted prefix, exactly like any other crash).
	r.mu.RLock()
	tb := r.tables["t1"]
	r.mu.RUnlock()
	tb.mu.RLock()
	fm := tb.history[0]
	tb.mu.RUnlock()
	fm.mu.RLock()
	cr := fm.reason
	fm.mu.RUnlock()
	if !strings.Contains(cr, "stalled") {
		t.Fatalf("match reason %q does not name the stall", cr)
	}
}

// TestStallGuardSetToZeroDoesNotHalt is the opt-out: the same stalled table
// with the guard EXPLICITLY at 0 keeps answering forever — the counter never
// trips because the guard is off — and only Close ends the match (aborted),
// never a halt.
func TestStallGuardSetToZeroDoesNotHalt(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.LoadDeck = stallLoader(t)
	o.MaxDecisionsPerTurn = 0 // explicit opt-out
	o.Seats = fourStallSeats
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")

	// The stall would run forever, so r.Wait would never return: instead poll
	// until the stalled loop has answered beyond the hand's 40 castable cards
	// (i.e. it is genuinely churning on repeatable activations, not about to
	// run dry), then Close and verify the ONLY thing that ended it was Close.
	deadline := time.Now().Add(30 * time.Second)
	for liveIntents(t, r, "t1") < 200 {
		if time.Now().After(deadline) {
			t.Fatalf("the stalled table never answered 200 decisions; it is not churning")
		}
		time.Sleep(time.Millisecond)
	}
	r.Close()
	r.Wait("t1")

	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchAborted {
		t.Fatalf("with the guard at 0 the match must end only by Close: %+v", ms)
	}
	if got := r.Tables()[0].State; got == protocol.TableHalted {
		t.Fatalf("table halted despite MaxDecisionsPerTurn 0")
	}
	if reason := tableReason(t, r, "t1"); reason != "" {
		t.Fatalf("halt reason %q despite the opt-out", reason)
	}
}

// TestANormalMatchPlaysToCompletionWithTheGuardAtItsDefault is the
// regression that matters: a guard that fires on a REAL game is worse than
// no guard, so a normal 4-seat match with the guard at the recommended
// default must play to completion untouched.
func TestANormalMatchPlaysToCompletionWithTheGuardAtItsDefault(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.MaxDecisionsPerTurn = DefaultMaxDecisionsPerTurn
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	_ = r.AddTable(fourSeatTable("t1", false))
	_ = r.Start("t1")
	r.Wait("t1")

	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("normal match with the guard at its default: %+v", ms)
	}
	if got := r.Tables()[0].State; got != protocol.TableIdle {
		t.Fatalf("table state %s after a finished match", got)
	}
}

// TestTheTurnAdvanceResetsThePerTurnCounter pins the RESET half of the
// guard: the count is per turn, so a game that advances turns must never
// trip, even with a limit below its total intent count. The legacy repo
// match below is measured to answer ~2019 intents over ~50 turns with a
// 68-decision busiest turn (fixed seed, fixed decks, fixed policy — a pure
// function of the config), so a limit of 100 sits above its busiest turn and
// far below its total: only a guard that fails to reset on turn advance
// could fire here.
func TestTheTurnAdvanceResetsThePerTurnCounter(t *testing.T) {
	t.Parallel()
	names := testutil.LegacyDeckNames()
	if len(names) < 4 {
		t.Skip("need at least 4 legacy repo decks")
	}
	o := testOptions(t)
	o.LoadDeck = repoDeckLoader(t)
	o.MaxDecisionsPerTurn = 100
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := TableConfig{ID: "t1", Name: "reset", Seats: 4, Decks: names[:4],
		Seed: 99, Pace: 0, Spectator: view.Omniscient, Mulligans: 1}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	r.Wait("t1")

	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchFinished || ms[0].Turns < 10 {
		t.Fatalf("turn-reset pin: %+v", ms)
	}
}

// TestAParkedHumanNeverTripsTheGuard is the count-vs-duration pin: a HumanSeat
// merely thinking is NOT an answered decision, so no amount of parking can
// advance the counter. With the limit set to 25 and the human parked on the
// very first decision of turn 1 (zero decisions answered), the guard must
// stay silent however long the human parks, and Close must end the match by
// abort, never by a stall crash.
func TestAParkedHumanNeverTripsTheGuard(t *testing.T) {
	t.Parallel()
	o := testOptions(t)
	o.MaxDecisionsPerTurn = 25 // a single answered decision should matter
	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	cfg := fourSeatTable("t1", false)
	cfg.Humans = []int{0}
	if err := r.AddTable(cfg); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("t1"); err != nil {
		t.Fatal(err)
	}
	// Seat 0 is the first to act on turn 1; wait until it has genuinely
	// parked (its slot is installed and answerable) — the human is thinking
	// now, and nothing else in the game can move: the loop is blocked on her.
	waitPending(t, r, "t1", 1, 0)

	// "However long it parks": leave it parked, watching the table stay live
	// the whole time. A wall-clock watchdog with a small budget would fire in
	// this window; the count-based guard has zero answered decisions to count.
	parkedUntil := time.Now().Add(time.Second)
	for time.Now().Before(parkedUntil) {
		if got := r.Tables()[0].State; got != protocol.TableLive {
			t.Fatalf("table left TableLive while a human was merely thinking (state %s)", got)
		}
		time.Sleep(50 * time.Millisecond)
	}
	tb := tableRef(t, r, "t1")
	tb.mu.RLock()
	live := tb.cur
	tb.mu.RUnlock()
	if live != nil {
		live.mu.RLock()
		st := live.state
		live.mu.RUnlock()
		if st != protocol.MatchLive {
			t.Fatalf("match %s while a human was parked — the guard must not count parked time", st)
		}
	}

	r.Close()
	r.Wait("t1")
	ms, _ := r.Matches("t1")
	if len(ms) != 1 || ms[0].State != protocol.MatchAborted {
		t.Fatalf("a parked human must end only by Close: %+v", ms)
	}
	if got := r.Tables()[0].State; got == protocol.TableHalted {
		t.Fatal("table halted while a human was merely thinking")
	}
	if reason := tableReason(t, r, "t1"); reason != "" {
		t.Fatalf("halt reason %q from a parked human", reason)
	}
}
