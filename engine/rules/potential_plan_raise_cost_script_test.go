package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPotentialPlanScriptsBeholdExileCastFromUncoveredMana pins the planner
// route that made Champion of the Clachan diverge at options in the std-ecl
// sweep: the spell's {3}{W} is payable only by first floating Springleaf
// Drum's any-colour mana (its "{T}, tap an untapped creature" activation is a
// source the payment census does not execute), and the RaiseCost BeholdExile
// part makes the ordinary cast planner decline the shape. The composed plan
// proves the census sources short, and the prefix search must still run: it
// activates the Drum, after which the cast is offered and the additional
// cost is paid normally. Before the fix the shape verdict was returned before
// the census/prefix search, so the play was "unsupported" and no cast option
// reached the oracle's full surface.
func TestPotentialPlanScriptsBeholdExileCastFromUncoveredMana(t *testing.T) {
	t.Parallel()
	e, _ := raiseEngine(t, []string{"Champion of the Clachan", "Thoughtweft Imbuer", "Springleaf Drum", "Dream Seizer", "Swamp"}, nil)
	champ := blightMove(t, e, 0, "Champion of the Clachan", state.ZHand)
	blightMove(t, e, 0, "Thoughtweft Imbuer", state.ZHand)
	blightMove(t, e, 0, "Springleaf Drum", state.ZBattlefield)
	seizer := blightMove(t, e, 0, "Dream Seizer", state.ZBattlefield)
	for i := 0; i < 3; i++ {
		blightMove(t, e, 0, "Swamp", state.ZBattlefield)
	}
	ppPriority(t, e)

	// The rule the plan depends on: with an empty pool the cast is not a raw
	// option, so only the scripted route can reach it; and the Drum (the one
	// source producing white) needs the untapped creature to pay.
	if raiseCastOffered(e, champ) {
		t.Fatal("precondition: Champion offered from the empty pool; the test needs the scripted route")
	}
	if o := e.G.Obj(seizer); o == nil || o.Tapped {
		t.Fatalf("precondition: Dream Seizer %v, want an untapped creature for the Drum", o)
	}

	pp := ppFind(t, e.PotentialPaymentPlans(0), "cast", champ)
	if pp.Reason != "" {
		t.Fatalf("Champion plan reason %q (%s), want a scripted witness", pp.Reason, pp.Detail)
	}
	if len(pp.Script) == 0 {
		t.Fatal("Champion plan has no script; the Drum-funded cast was written off before the prefix search")
	}

	// Replay the script's answers, then the cast must be offered from the
	// floated pool (the same pick replay granted_colour_offer_test uses).
	for i, st := range pp.Script {
		d := e.Pending()
		if d == nil || d.Kind != st.Kind {
			t.Fatalf("step %d: pending %v, want a %v decision", i, d, st.Kind)
		}
		in := decision.Intent{Seq: d.Seq, Player: d.Player}
		for _, pk := range st.Picks {
			found := -1
			for j := range d.Options {
				if pk.Matches(&d.Options[j]) && !slices.Contains(in.Choices, d.Options[j].Index) {
					found = d.Options[j].Index
					break
				}
			}
			if found < 0 {
				t.Fatalf("step %d: pick %q not offered", i, pk.Label)
			}
			in.Choices = append(in.Choices, found)
		}
		if err := e.Submit(in); err != nil {
			t.Fatalf("step %d: %v", i, err)
		}
	}
	// The scripted route the oracle lowers: once the uncovered source has
	// floated its mana, the planner pays the cast from the pool plus the
	// census sources it taps, so the play gains a witness plan.
	after := ppFind(t, e.PotentialPaymentPlans(0), "cast", champ)
	if after.Reason != "" || after.Plan == nil || len(after.Script) != 0 {
		t.Fatalf("after the script: Champion plan reason %q plan=%v script=%d, want the witness plan",
			after.Reason, after.Plan != nil, len(after.Script))
	}
}
