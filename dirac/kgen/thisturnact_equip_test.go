package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// TestThisTurnActEquipLowers: Count$ThisTurnActivated_Activated.Equip+YouCtrl
// (Kili the Resourceful's X) lowers to the turn-activation fact's Equip arm
// (N 1), beside the Exhaust arm (N 0) it shares an expression kind with; any
// other spec still refuses by name.
func TestThisTurnActEquipLowers(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}, known: func(string) bool { return false }}
	eq, err := lc.countValue("ThisTurnActivated_Activated.Equip+YouCtrl", 0)
	if err != nil || eq.Kind != vThisTurnActExhaust || eq.N != 1 {
		t.Fatalf("ThisTurnActivated equip = %+v err %v, want ThisTurnActExhaust N 1", eq, err)
	}
	ex, err := lc.countValue("ThisTurnActivated_Activated.Exhaust+YouCtrl", 0)
	if err != nil || ex.Kind != vThisTurnActExhaust || ex.N != 0 {
		t.Fatalf("ThisTurnActivated exhaust = %+v err %v, want ThisTurnActExhaust N 0", ex, err)
	}
	if eq.N == ex.N {
		t.Fatal("precondition: the Equip and Exhaust arms lower to the same value")
	}
	if _, err := lc.countValue("ThisTurnActivated_Activated.Cycling+YouCtrl", 0); err == nil || !strings.Contains(err.Error(), "ThisTurnActivated") {
		t.Fatalf("another ThisTurnActivated spec: want a named refusal, got %v", err)
	}
	cb := &cardBuilder{}
	for _, tc := range []struct {
		v lValue
		n int16
	}{{ex, 0}, {eq, 1}} {
		e, err := cb.expr(tc.v)
		if err != nil || e.Kind != kop.ExThisTurnActExhaust || e.N != tc.n {
			t.Fatalf("%+v builds to %+v err %v, want ExThisTurnActExhaust N %d", tc.v, e, err, tc.n)
		}
	}
}
