package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestPrefixScriptSkipsTheAbilitysOwnTapSource: Katilda, Dawnhart Prime's
// "{4}{G}{W}, {T}: put a +1/+1 counter on each creature you control" is
// payable with five lands and Elite Vanguard's granted mana, but a prefix
// that taps Katilda herself for mana (she is a Human too, and first in the
// battlefield order) leaves the play's own {T} unpayable. The search must
// not try her: the script that comes out activates the Vanguard, not the
// ability's source.
func TestPrefixScriptSkipsTheAbilitysOwnTapSource(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	board := []string{"Katilda, Dawnhart Prime", "Elite Vanguard", "Forest", "Forest", "Plains", "Plains", "Swamp"}
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
	var katilda state.ObjID
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if e.G.Obj(id).Face().Name == "Katilda, Dawnhart Prime" {
			katilda = id
		}
	}
	found := false
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind != "ability" || pp.Action.Obj != katilda {
			continue
		}
		found = true
		if pp.Reason != "" || len(pp.Script) == 0 {
			t.Fatalf("Katilda's counters ability: reason %q detail %q script %d steps, want a script", pp.Reason, pp.Detail, len(pp.Script))
		}
		for _, st := range pp.Script {
			for _, pk := range st.Picks {
				if pk.Kind == "activate" && pk.Obj == katilda {
					t.Fatalf("the script taps Katilda for mana, leaving her own {T} cost unpayable: %+v", pp.Script)
				}
			}
		}
	}
	if !found {
		t.Fatal("Katilda's counters ability is not a potential play")
	}
}
