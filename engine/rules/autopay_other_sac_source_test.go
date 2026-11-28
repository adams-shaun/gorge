package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// An activation whose cost removes a counter from its own source (Academy
// Elite's "{2}{U}, Remove a +1/+1 counter from Academy Elite: Draw a card")
// must keep that source on the battlefield: an Ashnod's Altar step that
// sacrificed it would leave the counter part unpayable (CR 602.2b, 601.2h).

const (
	otherSacEliteSrc  = "Name:OtherSac Elite\nManaCost:1 U\nTypes:Creature\nPT:0/0\nA:AB$ Draw | Cost$ 2 U SubCounter<1/P1P1> | NumCards$ 1 | SpellDescription$ x\nOracle:x\n"
	otherSacIslandSrc = "Name:OtherSac Island\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ U | SpellDescription$ x\nOracle:x\n"
	otherSacBigSrc    = "Name:OtherSac Big\nManaCost:3\nTypes:Creature\nPT:3/3\nOracle:x\n"
)

// otherSacElitePlan is the potential plan of elite's activated ability.
func otherSacElitePlan(t *testing.T, e *Engine, elite state.ObjID) PotentialPlan {
	t.Helper()
	ppPriority(t, e)
	for _, pp := range e.PotentialPaymentPlans(0) {
		if pp.Action.Kind == "ability" && pp.Action.Obj == elite {
			return pp
		}
	}
	t.Fatalf("no potential plan for the elite's ability: %+v", e.PotentialPaymentPlans(0))
	return PotentialPlan{}
}

func otherSacEliteBoard(t *testing.T, seed uint64) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e, _, _ := newFixtureDeck(t, seed, otherSacSpellSrc)
	altar := onBoard(t, e, 0, otherSacAltarSrc)
	onBoardReady(t, e, 0, otherSacIslandSrc)
	elite := onBoardReady(t, e, 0, otherSacEliteSrc)
	e.G.Obj(elite).AddCounter("P1P1", 1)
	return e, altar, elite
}

func TestAutopayOtherSacKeepsTheActivatingSource(t *testing.T) {
	t.Parallel()
	t.Run("only creature", func(t *testing.T) {
		e, altar, elite := otherSacEliteBoard(t, 4721)
		if e.G.Obj(elite).Zone != state.ZBattlefield || e.G.Obj(elite).Counter("P1P1") != 1 {
			t.Fatalf("precondition: elite zone %s counters %v, want battlefield with one +1/+1", e.G.Obj(elite).Zone, e.G.Obj(elite).Counters)
		}
		pp := otherSacElitePlan(t, e, elite)
		if pp.Plan == nil {
			return
		}
		for _, a := range pp.Plan.Activations {
			if a.Source == altar {
				for _, v := range a.Sacrifices {
					if v == elite {
						t.Fatalf("plan %+v sacrifices the activating elite to the altar, whose counter the cost removes", pp.Plan.Activations)
					}
				}
			}
		}
		t.Fatalf("plan %+v (reason %q), want no automatic plan: the elite is the only victim", pp.Plan.Activations, pp.Reason)
	})
	t.Run("second creature", func(t *testing.T) {
		e, altar, elite := otherSacEliteBoard(t, 4722)
		big := onBoardReady(t, e, 0, otherSacBigSrc)
		// Precondition: the ranking alone prefers the elite (lower mana value).
		if mvE, mvB := state.ManaValue(e.G, e.G.Obj(elite)), state.ManaValue(e.G, e.G.Obj(big)); !(mvE < mvB) {
			t.Fatalf("precondition: elite MV %d, big MV %d, want the elite lower", mvE, mvB)
		}
		pp := otherSacElitePlan(t, e, elite)
		if pp.Plan == nil || pp.Reason != "" {
			t.Fatalf("potential plan = reason %q detail %q plan %v, want a witness", pp.Reason, pp.Detail, pp.Plan)
		}
		var victims []state.ObjID
		for _, a := range pp.Plan.Activations {
			if a.Source == altar {
				victims = append(victims, a.Sacrifices...)
			}
		}
		if len(victims) != 1 || victims[0] != big {
			t.Fatalf("altar victims = %v, want the second creature %d (elite %d)", victims, big, elite)
		}
	})
}
