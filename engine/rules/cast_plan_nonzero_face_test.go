package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// castPlanRoomSrc is an authored Room whose two doors have different costs,
// so the face the card shows decides what the cast owes: door 0 is {1}{B},
// door 1 is {B}. A Room that left the battlefield as its other door keeps
// that door in the hand, and CR 709.3/309.4b lets the player cast the half
// it shows.
const castPlanRoomSrc = "Name:Planner Room Front\nManaCost:1 B\nTypes:Enchantment Room\nOracle:front\n" +
	"ALTERNATE\nName:Planner Room Back\nManaCost:B\nTypes:Enchantment Room\nA:SP$ Draw | NumCards$ 1\nOracle:back\nAlternateMode:Split\n"

// TestCastPlanPlansNonzeroFace: a Room back in hand showing door 1, one
// untapped Swamp and an empty pool. The ordinary cast of the shown face is
// the cheap door ({B}), so it must be offered through the planned payment
// surface (the plain offer walk is float-gated with an empty pool) and the
// witness must tap the Swamp and actually cast it.
func TestCastPlanPlansNonzeroFace(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 77501, castPlanRoomSrc)
	swamp := onBoard(t, e, 0, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n")
	toMain1(t, e)
	o := e.G.Obj(id)
	if o == nil || o.Zone != state.ZHand || o.FaceIdx != 0 {
		t.Fatalf("precondition: Room in hand at face 0, got %+v", o)
	}
	// The room left the battlefield as its back door: the shown face is 1.
	o.SetFaceIdx(1)
	if got := o.Face(); got == nil || got.Name != "Planner Room Back" {
		t.Fatalf("precondition: shown face = %v, want the back door", got)
	}
	if pool := e.G.Players[0].Pool; pool.Total() != 0 {
		t.Fatalf("precondition: pool %v not empty", pool)
	}
	got := e.PlanCastPayment(0, decision.PlannedCast{Object: id, Face: int(o.FaceIdx), Origin: "hand"})
	if got.Plan == nil || got.Reason != "" {
		t.Fatalf("plain nonzero-face verdict %+v, want a witness from the untapped Swamp", got)
	}
	if len(got.Plan.Activations) != 1 || got.Plan.Activations[0].Source != swamp {
		t.Fatalf("witness %s, want the Swamp %d", lrPlanString(got.Plan.Activations), swamp)
	}
	d := paymentPlanReask(t, e)
	a := ppAction(t, d, id)
	if a.Cast.Face != 1 {
		t.Fatalf("planned cast face = %d, want the shown face 1", a.Cast.Face)
	}
	submitPaymentPlan(t, e, d, a)
	for guard := 0; guard < 20; guard++ {
		cur := e.Pending()
		if cur == nil || cur.Kind == decision.KPriority {
			break
		}
		if err := e.Submit(decision.Intent{Seq: cur.Seq, Player: cur.Player, Choices: []int{cur.Options[0].Index}}); err != nil {
			t.Fatal(err)
		}
	}
	if z := e.G.Obj(id).Zone; z != state.ZStack && z != state.ZBattlefield {
		t.Fatalf("the planned cast left the Room in %s, want the stack (or resolved)", z)
	}
	if !e.G.Obj(swamp).Tapped {
		t.Fatalf("the witness paid no mana: Swamp %d is untapped", swamp)
	}
}

// TestCastPlanPlansNonzeroFacePotential is the potential-walk companion:
// both the shown door's plain cast and the other door are priced from the
// untapped Swamp on an empty pool.
func TestCastPlanPlansNonzeroFacePotential(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 77502, castPlanRoomSrc)
	var swamps []state.ObjID
	for range 2 {
		swamps = append(swamps, onBoard(t, e, 0, "Name:Swamp\nTypes:Basic Land Swamp\nOracle:x\n"))
	}
	toMain1(t, e)
	o := e.G.Obj(id)
	o.SetFaceIdx(1)
	ppPriority(t, e)
	pps := e.PotentialPaymentPlans(0)
	for _, c := range []struct {
		mode string
		n    int
	}{{"", 1}, {"room_alt", 2}} {
		pp := faceCastPlan(t, pps, id, c.mode)
		if pp.Plan == nil || pp.Reason != "" {
			t.Fatalf("%s verdict %+v, want a witness from the Swamps", c.mode, pp)
		}
		if len(pp.Plan.Activations) != c.n {
			t.Fatalf("%s witness %s, want %d untapped Swamps", c.mode, lrPlanString(pp.Plan.Activations), c.n)
		}
		for _, a := range pp.Plan.Activations {
			found := false
			for _, src := range swamps {
				found = found || a.Source == src
			}
			if !found {
				t.Fatalf("%s witness tapped %d, want only the untapped Swamps", c.mode, a.Source)
			}
		}
	}
}
