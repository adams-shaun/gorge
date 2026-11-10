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
	if _, ok := grantedWarpCost(e, id); !ok {
		t.Fatalf("no granted warp cost; derived keywords %v", e.Keywords(id))
	}
	// Two mana pays the printed {1}{R} but not the granted {2}{R}.
	addMana(t, e, 0, "R")
	addMana(t, e, 0, "R")
	if grantedWarpOffered(e, id) {
		t.Fatal("warped row offered with only 2 mana floating; the grant costs {2}{R}")
	}
	addMana(t, e, 0, "R")
	if got := e.G.Players[0].Pool.Total(); got != 3 {
		t.Fatalf("pool before the warp cast: %d, want the 3 floated", got)
	}
	submitChoices(t, e, castModeOption(t, e, id, "warped"))
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(id)
	if o.Zone != state.ZBattlefield || o.CastFlags&state.FlagWarped == 0 {
		t.Fatalf("Goblin Piker zone %s flags %d, want a warped permanent", o.Zone, o.CastFlags)
	}
	if got := e.G.Players[0].Pool.Total(); got != 0 {
		t.Fatalf("pool after the {2}{R} warp cast: %v (3 floated, the granted cost charges all 3)", e.G.Players[0].Pool)
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

// TestGrantedWarpCheaperInstanceWins: a card that prints Warp and is granted
// another cost prices its warp row at the cheaper instance. Red Tiger Mechan
// prints {1}{R} (cheaper than Tannuk's {2}{R}); Spirited Simulacrum prints {4}
// (dearer), so the grant's {2}{R} is the row.
func TestGrantedWarpCheaperInstanceWins(t *testing.T) {
	t.Parallel()
	e, _, _ := altCostEngine(t, 954, []string{"Tannuk, Steadfast Second", "Red Tiger Mechan", "Spirited Simulacrum"}, nil, nil)
	findCardObj(t, e, 0, "Tannuk, Steadfast Second", state.ZBattlefield)
	m := altCastFor("warped")
	for _, tc := range []struct {
		name string
		size int32
	}{{"Red Tiger Mechan", 2}, {"Spirited Simulacrum", 3}} {
		id := findCardObj(t, e, 0, tc.name, state.ZHand)
		f := e.G.Obj(id).Face()
		if _, printed := f.KeywordParam("Warp"); !printed {
			t.Fatalf("%s prints no Warp", tc.name)
		}
		if c, ok := altRowCost(e, m, id, f); !ok || costSize(c) != tc.size {
			t.Fatalf("%s warp row cost %v ok=%v, want size %d", tc.name, c, ok, tc.size)
		}
	}
	id := findCardObj(t, e, 0, "Spirited Simulacrum", state.ZHand)
	addMana(t, e, 0, "R")
	addMana(t, e, 0, "R")
	if grantedWarpOffered(e, id) {
		t.Fatal("warped row offered with 2 mana; the cheaper instance costs {2}{R}")
	}
	addMana(t, e, 0, "R")
	if !grantedWarpOffered(e, id) {
		t.Fatal("warped row not offered at the granted {2}{R} with 3 mana floating")
	}
}
