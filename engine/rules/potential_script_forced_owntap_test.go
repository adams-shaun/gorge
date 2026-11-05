package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPrefixScriptSkipsAForcedTapOfTheAbilitysOwnSource: Guy in the Chair's
// "{2}{G}, {T}: put a +1/+1 counter on target Spider" is payable with two
// Plains and Supportive Parents ("Tap two untapped creatures you control: Add
// one mana of any color") tapping two creatures other than Guy. With four
// creatures the first activation asks which two to tap; the second has only
// two candidates left, Guy among them, so nothing is asked and both are
// tapped, which the ask-only skip cannot see. The line that ends there leaves
// Guy's own {T} unpayable; the search must go on to a line that does not.
func TestPrefixScriptSkipsAForcedTapOfTheAbilitysOwnSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	board := []string{"Guy in the Chair", "Spider-Ham, Peter Porker", "Grizzly Bears", "Supportive Parents", "Plains", "Plains"}
	var deck []*cards.Card
	for _, n := range board {
		deck = append(deck, lookup(t, reg, n))
	}
	cfg := Config{Seed: 7, Names: []string{"a", "b"},
		Decks:  [][]*cards.Card{append(deck, mountainDeck(t, 40-len(deck))...), mountainDeck(t, 40)},
		Tokens: reg.Tokens,
	}
	e := New(seatZeroStart(cfg))
	e.Advance()
	for _, n := range board {
		moveToBattlefield(t, e, findAndMoveToHand(t, e, 0, n))
	}
	e.pending = nil
	e.Advance()
	driveToStep(t, e, 3, 0, state.StepMain1)
	var guy state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		o := e.G.Obj(id)
		if o.Face().Name == "Guy in the Chair" {
			guy = id
		}
		o.SummonSick = false // summoning sickness is not what this tests
	}
	found := false
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "ability" || pp.Action.Obj != guy || pp.Action.Ability != 1 {
			continue
		}
		found = true
		if pp.Reason != "" || len(pp.Script) == 0 {
			t.Fatalf("Guy's counters ability: reason %q detail %q script %d steps, want a script", pp.Reason, pp.Detail, len(pp.Script))
		}
		acts := 0
		for _, st := range pp.Script {
			for _, pk := range st.Picks {
				if pk.Kind == "tapcost" && pk.Obj == guy {
					t.Fatalf("the script taps Guy for Supportive Parents' cost, leaving his own {T} unpayable: %+v", pp.Script)
				}
				if pk.Kind == "activate" {
					acts++
				}
			}
		}
		// Every second activation would tap Guy (two candidates, no ask), so the
		// script that pays has exactly one.
		if acts != 1 {
			t.Fatalf("the script activates a mana source %d times, want 1: %+v", acts, pp.Script)
		}
	}
	if !found {
		t.Fatal("Guy's counters ability is not a potential play")
	}
}
