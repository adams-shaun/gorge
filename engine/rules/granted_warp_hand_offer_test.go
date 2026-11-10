package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestGrantedWarpHandOffer: Tannuk, Steadfast Second gives red creature cards
// in hand warp {2}{R}. The granted card is offered as a "warped" cast at that
// cost, is exiled at the next end step, and is recast from exile on a later
// turn for its normal cost.
func TestGrantedWarpHandOffer(t *testing.T) {
	t.Parallel()
	e, cfg, _ := altCostEngine(t, 953, []string{"Tannuk, Steadfast Second", "Goblin Piker", "Sinister Cryologist"}, nil, nil)
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	id := findCardObj(t, e, 0, "Goblin Piker", state.ZHand)
	// Precondition: the grant is on the derived keywords, not the printed face.
	if _, printed := e.G.Obj(id).Face().KeywordParam("Warp"); printed {
		t.Fatal("Goblin Piker prints Warp; the test needs a granted warp")
	}
	if _, ok := e.grantedWarpCost(id); !ok {
		t.Fatalf("no granted warp cost; derived keywords %v", e.Keywords(id))
	}
	// Two mana pays the printed {1}{R} but not the granted {2}{R}.
	addMana(t, e, 0, "R")
	addMana(t, e, 0, "R")
	if grantedWarpOffered(e, id) {
		t.Fatal("warped row offered with only 2 mana floating; the grant costs {2}{R}")
	}
	addMana(t, e, 0, "R")
	submitChoices(t, e, castModeOption(t, e, id, "warped"))
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped == 0 {
		t.Fatalf("Goblin Piker zone %s flags %d, want a warped permanent", o.Zone, o.CastFlags)
	}
	if got := e.G.Players[0].ManaUnits(); got != ([7]state.Mana{}) {
		t.Fatalf("mana left after the {2}{R} warp cast: %v", got)
	}
	driveToStepAll(t, e, e.G.Turn, 0, state.StepEnd)
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZExile {
		t.Fatalf("Goblin Piker after the end step: %s, want exile", o.Zone)
	}
	driveToStepAll(t, e, e.G.Turn+2, 0, state.StepMain1)
	addMana(t, e, 0, "R")
	addMana(t, e, 0, "R")
	submitChoices(t, e, castModeOption(t, e, id, "warp_recast"))
	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped != 0 {
		t.Fatalf("Goblin Piker after the recast: zone=%s flags=%d", o.Zone, o.CastFlags)
	}
	replayCheck(t, e, cfg)
}

func grantedWarpOffered(e *Engine, id state.ObjID) bool {
	d := e.Pending()
	if d == nil || d.Kind != decision.KPriority {
		return false
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == id && o.Mode == "warped" {
			return true
		}
	}
	return false
}
