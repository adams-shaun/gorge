package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTripleAmountResolvesPerAffectedObject pins Forge's bare `NumAtt$ Triple`
// / `NumDef$ Triple` amount (Tifa's Limit Break, Final Heaven). Forge's
// PumpEffect codes the ADDED amount as twice the object's own characteristic
// (final 3x), resolved per affected object like Double. The board holds two
// creatures of different power (Bear 2/2, Flier 1/1) so a collapsed scalar
// cannot pass; a wrong Triple -> 3 mapping gives {6,6}/{3,3} and fails.
func TestTripleAmountResolvesPerAffectedObject(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
	}{
		{"PumpDefinedValid", "DB$ Pump | Defined$ Valid Creature.YouCtrl | NumAtt$ Triple | NumDef$ Triple"},
		{"PumpAllValidCards", "DB$ PumpAll | ValidCards$ Creature.YouCtrl | NumAtt$ Triple | NumDef$ Triple"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g, ids := board(t)
			h := &fakeHost{g: g}
			if bp, fp := h.Power(ids["myBear"]), h.Power(ids["myFlier"]); bp == fp {
				t.Fatalf("precondition: bear/flier power must differ, both %d", bp)
			}
			Resolve(h, &Ctx{Controller: 0}, sa(t, tc.body))
			got := map[state.ObjID][2]int32{}
			for _, ce := range h.continuous {
				if ce.Layer == state.LPT {
					got[ce.Source] = [2]int32{ce.AddPower, ce.AddToughness}
				}
			}
			if len(got) != 2 {
				t.Fatalf("P/T registrations = %d, want 2 (one per affected creature): %+v", len(got), h.continuous)
			}
			if want := [2]int32{4, 4}; got[ids["myBear"]] != want {
				t.Errorf("bear Triple = %v, want %v (adds twice its own 2/2)", got[ids["myBear"]], want)
			}
			if want := [2]int32{2, 2}; got[ids["myFlier"]] != want {
				t.Errorf("flier Triple = %v, want %v (adds twice its own 1/1)", got[ids["myFlier"]], want)
			}
		})
	}
}

// TestTripleAmountScalarDegradesToZero keeps the fail-closed scalar contract
// for Triple, as TestDoubleAmountScalarDegradesToZero does for Double.
func TestTripleAmountScalarDegradesToZero(t *testing.T) {
	g, _ := board(t)
	h := &fakeHost{g: g}
	if got := Num(h, &Ctx{}, sa(t, "SP$ Pump | NumAtt$ Triple"), "NumAtt", 7); got != 0 {
		t.Fatalf("scalar NumAtt$ Triple = %d, want 0", got)
	}
	if got := NumForObject(h, &Ctx{}, sa(t, "SP$ Pump | NumAtt$ Triple"), "NumAtt", 0, 0); got != 0 {
		t.Fatalf("NumForObject NumAtt$ Triple with no object = %d, want 0", got)
	}
}
