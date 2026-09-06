package host

// Task M2c-2: TableConfig.Humans — reproducible human-driven single-shot
// tables.
//
// Covers the two halves of the new surface that matter for the M2e fixture
// chain:
//
//  1. a table with Humans seated plays exactly one match and then stops
//     (single-shot) even though no human ever answers a decision — the
//     human seat must neither stall play nor drag the table into autoplay;
//  2. validation is a real validation: out-of-range, equal-to-seat-count,
//     negative and duplicated human indices, and a Perpetual+Humans
//     combination, are all refused by AddTable with an error (never a panic,
//     never a half-created table), while a valid Humans plan is accepted.
//
// Test 1 proves the ThinkTimeout path in the way only absence of the person
// can: the human seat is left unsettled (never SubmitIntent-driven), so the
// match can only complete because HumanSeat's deterministic caretaker fired.
// It then asserts single-shotness directly — one match in history, table
// back to TableIdle, no second match autoplayed. Reading the caretaker
// counter off the finished match's slot is the one honest signal that the
// timeout really did the answering, because by design D3 a caretaker intent
// and a human intent are byte-identical.

import (
	"testing"
	"time"

	"github.com/adams-shaun/gorge/protocol"
	"github.com/adams-shaun/gorge/view"
)

// twoSeatHumanTable is a 2-seat table (slot 0 human, slot 1 bot) — the
// smallest plan the brief's "Humans: []int{0} and one bot" describes. Seat 1
// stays a bot both to keep the game short enough for the package budget and
// to prove the run loop seats the complement with bots, exactly as the
// pure-bot default would.
func twoSeatHumanTable(id TableID) TableConfig {
	return TableConfig{ID: id, Name: "Table " + string(id), Seats: 2, Decks: []string{"a", "b"},
		Seed: 99, Pace: 0, Spectator: view.Omniscient, Perpetual: false, Humans: []int{0}}
}

// TestHumanSeatedTableHonorsThinkTimeoutAndEndsSingleShot is Task M2c-2 step
// 1+3 rolled together: no human ever answers, a real ThinkTimeout lets the
// caretaker drive the table, and the table is single-shot. Deliberately not
// t.Parallel: it owns a match slot and its wall cost is bounded by the
// ThinkTimeout × number of decisions, so it runs in the sequential phase.
func TestHumanSeatedTableHonorsThinkTimeoutAndEndsSingleShot(t *testing.T) {
	o := testOptions(t)
	// Small but not pathological: the human is unspecified, so every decision
	// it is asked parks until the caretaker deadline; a value here balances
	// speed against the timer actually firing under load.
	o.ThinkTimeout = time.Millisecond

	r, err := New(o)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	if err := r.AddTable(twoSeatHumanTable("hn")); err != nil {
		t.Fatal(err)
	}
	if err := r.Start("hn"); err != nil {
		t.Fatal(err)
	}
	r.Wait("hn")

	// (1) Single-shot: exactly one finished match, table idle again — the
	// run loop must not autoplay a next match behind the human.
	ti := r.Tables()[0]
	if ti.State != protocol.TableIdle || ti.Match != 1 {
		t.Fatalf("table after human match: %+v, want idle after 1 match", ti)
	}
	ms, err := r.Matches("hn")
	if err != nil || len(ms) != 1 || ms[0].State != protocol.MatchFinished {
		t.Fatalf("matches after human table: %+v (err %v), want one finished", ms, err)
	}

	// (2) The human seat was a real HumanSeat whose caretaker answered every
	// decision (the idle human was unsettled — nothing drove it). Only
	// the caretaker counter can distinguish this from a human, because D3
	// makes the intents byte-identical.
	r.mu.RLock()
	tb := r.tables["hn"]
	r.mu.RUnlock()
	tb.mu.RLock()
	var fm *match
	if len(tb.history) > 0 {
		fm = tb.history[0]
	}
	tb.mu.RUnlock()
	if fm == nil {
		t.Fatal("the finished human match never landed in table history")
	}
	hs, ok := fm.slots[0].(*HumanSeat)
	if !ok {
		t.Fatalf("slot 0 is %T, want a *HumanSeat for a Humans config", fm.slots[0])
	}
	if got := hs.caretakerCount(); got == 0 {
		t.Fatal("caretaker never fired yet the human never answered: the match should not have completed")
	}
	if _, isBot := fm.slots[1].(*HumanSeat); isBot {
		t.Fatal("slot 1 (not in Humans) was seated with a human; it must stay a bot")
	}
	t.Logf("human table: %d events, %d turns, %d caretaker answers", ms[0].Events, ms[0].Turns, hs.caretakerCount())
}

// TestHumanSeatValidation exercises both directions. The negative side is the
// one that matters: a bad Humans plan must be refused by AddTable with an
// error — not a panic, not a table half-acted. Every rejected case is also
// checked to leave the registry without the table, so no plan can be
// half-applied.
func TestHumanSeatValidation(t *testing.T) {
	t.Parallel()

	// Happy path: a multi-human plan within a 4-seat table is accepted and
	// surfaces on the overview.
	r, err := New(testOptions(t))
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	good := fourSeatTable("ok", false)
	good.Humans = []int{0, 2}
	if err := r.AddTable(good); err != nil {
		t.Fatalf("valid Humans plan rejected: %v", err)
	}

	bad := []struct {
		name string
		seat int
	}{
		{"out of range", 9},
		{"equals seat count", 4},
		{"negative", -1},
		{"duplicate", 1},
	}
	for _, bc := range bad {
		cfg := fourSeatTable("x", false)
		cfg.Humans = []int{bc.seat, 1} // the duplicate case: [1,1]; the others exercise their named bound
		if err := r.AddTable(cfg); err == nil {
			t.Errorf("%s: AddTable accepted Humans %v on seats %d", bc.name, cfg.Humans, cfg.Seats)
		}
	}
	// Perpetual + Humans is the flagged interaction (see brief addendum): it
	// is rejected, never silently forced to single-shot.
	conflict := fourSeatTable("x", true)
	conflict.Humans = []int{0}
	if err := r.AddTable(conflict); err == nil {
		t.Fatal("Perpetual+Humans combination accepted")
	}

	// None of the rejected tables half-created.
	if got := r.Tables(); len(got) != 1 {
		t.Fatalf("registry has %d tables after rejects, want only the accepted one: %+v", len(got), got)
	}
}
