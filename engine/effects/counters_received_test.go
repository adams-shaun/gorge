package effects

import "testing"

// ledgerHost is a fakeHost that also publishes a per-turn counter ledger, the
// optional role the engine implements (rules.Engine.CounterAddsThisTurn).
type ledgerHost struct {
	*fakeHost
	adds []CounterAdd
}

func (h ledgerHost) CounterAddsThisTurn() []CounterAdd { return h.adds }

// TestCountersReceivedThisTurnPredicate holds the La Forge shape
// (Permanent.YouCtrl+countersReceivedThisTurn_GE1_Any_You): the object is
// matched by id and the adder by player, "Any" sums every kind, and a host
// that publishes no ledger reads as no counters put. The turn-change reset is
// the engine's ledger lifetime, held in engine/rules.
func TestCountersReceivedThisTurnPredicate(t *testing.T) {
	const spec = "Permanent.YouCtrl+countersReceivedThisTurn_GE1_Any_You"
	h, g, src := playersBoard(t, 2, mkCard(t, "Name:Forge\nTypes:Creature Human\nPT:2/4\nOracle:x\n"))
	mine := addPermanent(t, h, 0, "Creature")
	minePlain := addPermanent(t, h, 0, "Creature")
	theirs := addPermanent(t, h, 1, "Creature")

	count := func(host Host, s string) int32 {
		t.Helper()
		n, ok := EvalCountOK(host, &Ctx{Source: src, Controller: 0}, "Count$Valid "+s)
		if !ok {
			t.Fatalf("Count$Valid %s was not evaluated", s)
		}
		return n
	}

	// Precondition: all four permanents are on the battlefield, so the zeroes
	// below are the word's verdict, not an empty board.
	if n := count(h, "Permanent"); n != 4 {
		t.Fatalf("battlefield permanents = %d, want 4", n)
	}
	// No ledger at all: nothing matches (and the word is recognised, not unknown).
	if n := count(h, spec); n != 0 {
		t.Errorf("host without a ledger counted %d, want 0", n)
	}
	if un := UnknownPredicates(spec); len(un) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v, want empty", spec, un)
	}

	lh := ledgerHost{fakeHost: h, adds: []CounterAdd{
		{Adder: 0, Obj: mine, Kind: "P1P1", Amount: 1},
		{Adder: 0, Obj: mine, Kind: "LORE", Amount: 2},      // Any: kinds sum, still one permanent
		{Adder: 1, Obj: minePlain, Kind: "P1P1", Amount: 1}, // an opponent's mark does not count for You
		{Adder: 0, Obj: theirs, Kind: "P1P1", Amount: 1},    // my mark on their permanent: not YouCtrl
	}}
	if n := count(lh, spec); n != 1 {
		t.Errorf("La Forge shape counted %d, want 1 (only my permanent I marked)", n)
	}
	// The object is matched by id and the sum is compared: GE3 needs both kinds.
	if n := count(lh, "Permanent.YouCtrl+countersReceivedThisTurn_GE3_Any_You"); n != 1 {
		t.Errorf("GE3 counted %d, want 1 (1+2 across two kinds)", n)
	}
	if n := count(lh, "Permanent.YouCtrl+countersReceivedThisTurn_GE4_Any_You"); n != 0 {
		t.Errorf("GE4 counted %d, want 0", n)
	}
	// EQ0 is true of every permanent I control that I did not mark.
	if n := count(lh, "Permanent.YouCtrl+countersReceivedThisTurn_EQ0_Any_You"); n != 2 {
		t.Errorf("EQ0 counted %d, want 2 (source and the unmarked creature)", n)
	}
	// An empty ledger (a new turn) matches nothing.
	if n := count(ledgerHost{fakeHost: h}, spec); n != 0 {
		t.Errorf("empty ledger counted %d, want 0", n)
	}
	_ = g
}

// TestCountersReceivedThisTurnShapesFailClosed pins the shapes this build does
// not read: they stay unknown by name (never silently false), so a card
// carrying one is refused rather than run with a dead filter.
func TestCountersReceivedThisTurnShapesFailClosed(t *testing.T) {
	for _, spec := range []string{
		"Creature.YouCtrl+countersReceivedThisTurn_GE1_P1P1_You", // Kid Loki: a continuous-effect scope, no ledger bound
		"Permanent.countersReceivedThisTurn_GE1_Any_Player",
		"Permanent.countersReceivedThisTurn_GE1_Any_Opp",
		"Permanent.countersReceivedThisTurn_GEX_Any_You",
		"Permanent.countersReceivedThisTurn_GE1_Any",
		"Permanent.countersReceivedThisTurn_XX1_Any_You",
	} {
		if un := UnknownPredicates(spec); len(un) != 1 {
			t.Errorf("UnknownPredicates(%q) = %v, want the one unread word", spec, un)
		}
	}
}
