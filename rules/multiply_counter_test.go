package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// api:MultiplyCounter pinned end to end on the in-deck real corpus carrier
// Lily Bowen, Raging Grandma. The brief (agent-20260919T185012Z-e1d44eb1):
// `DB$ MultiplyCounter` was unregistered, so Lily's power<=16 upkeep branch
// (SVar:DBPump:DB$ MultiplyCounter | CounterType$ P1P1 | Defined$ Self) did
// nothing -- the "double the number of +1/+1 counters" half of the card.
//
// Lily enters with two +1/+1 counters (K:etbCounter:P1P1:2), is 0/0 so its
// power with two counters is 2 (<= 16), and its upkeep trigger runs
// TrigBranch: BranchConditionSVar$ Power GT16 is false -> DBPump. The fix
// emits a CounterChange adding (Multiplier-1) x current = 2, so the count
// goes 2 -> 4 and the event stream records the real +2 change.

// lilyEngine deals seat 0 a deck whose first card is Lily Bowen and drives to
// seat 0's Main1. The rest of the deck is Mountains so the shuffle is
// deterministic and no other card interferes with the upkeep trigger.
func lilyEngine(t *testing.T, reg *cards.Registry) *Engine {
	t.Helper()
	mountain := searchCorpusCard(t, reg, "Mountain")
	deck := []*cards.Card{searchCorpusCard(t, reg, "Lily Bowen, Raging Grandma")}
	for len(deck) < 40 {
		deck = append(deck, mountain)
	}
	opp := make([]*cards.Card, 40)
	for i := range opp {
		opp[i] = mountain
	}
	cfg := seatZeroStart(Config{Seed: 6119, Names: []string{"lily", "opponent"},
		Decks: [][]*cards.Card{deck, opp}, Tokens: reg.Tokens})
	e := New(cfg)
	e.Advance()
	toMain1(t, e)
	return e
}

// TestLilyBowenMultiplyCounterDoublesAtUpkeep drives the real corpus card and
// asserts the emitted counter change, not just the post-state.
func TestLilyBowenMultiplyCounterDoublesAtUpkeep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := lilyEngine(t, reg)

	id := searchMoveByName(t, e, "Lily Bowen, Raging Grandma", state.ZBattlefield)
	lily := e.G.Obj(id)
	if lily == nil || lily.Zone != state.ZBattlefield {
		t.Fatalf("Lily not on the battlefield: %+v", lily)
	}
	if got := lily.Counter("P1P1"); got != 2 {
		t.Fatalf("Lily entered with %d P1P1, want 2 (K:etbCounter:P1P1:2)", got)
	}

	// Count Lily's +2 P1P1 CounterChanges logged by the entry itself, so the
	// doubling assertion measures the DELTA the upkeep trigger adds (the entry
	// PutCounter and MultiplyCounter both emit Amount 2).
	entryAdds := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == id && ev.Counter == "P1P1" && ev.Amount == 2 {
			entryAdds++
		}
	}

	// Seat 0's next upkeep is turn 3 (turn 1 seat 0, turn 2 seat 1).
	driveToStep(t, e, 3, 0, state.StepUpkeep)
	passUntilStackEmpty(t, e, 40)

	if ec := e.G.Obj(id).Counter("P1P1"); ec != 4 {
		t.Fatalf("Lily has %d P1P1 after the upkeep trigger, want 4 (2 doubled)", ec)
	}

	// The brief requires the counter CHANGE on the event stream, not only the
	// final count: one MORE +2 P1P1 CounterChange than the entry logged.
	doublingAdds := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == id && ev.Counter == "P1P1" && ev.Amount == 2 {
			doublingAdds++
		}
	}
	if got := doublingAdds - entryAdds; got != 1 {
		t.Fatalf("logged %d New CounterChange(P1P1, +2) on Lily (%d total, %d entry), want exactly 1 (MultiplyCounter's (Multiplier-1)*current)", got, doublingAdds, entryAdds)
	}
}
