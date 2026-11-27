package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCutADealLowering: Cut a Deal's "you draw a card for each opponent who
// drew a card this way" (X = PlayerCountOpponents$HasPropertyIsRememberedOrController)
// lowers to the remembered-set read, behind the opponents' RememberDrawn$ Draw.
// The count is the general rem-ctrl read (kop.OppExtRemCtrl; dirac
// value_remctrl.go), exact at every seat count, so the def is NOT two-seat —
// main's two-seat reduction of the same body was superseded by it.
func TestCutADealLowering(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Cut a Deal"})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	idx := -1
	for i, c := range all {
		if !c.Token && c.Name == "Cut a Deal" {
			idx = i
		}
	}
	if idx < 0 {
		t.Fatal("Cut a Deal: not lowered")
	}
	if r := lowerRefusals(all[idx]); len(r) > 0 {
		t.Fatalf("Cut a Deal: refused: %v", r)
	}
	def := bs[idx].def
	if def.TwoSeat {
		t.Error("Cut a Deal: marked two-seat; the general rem-ctrl read seats any count")
	}
	var draws []kop.Instr
	for _, a := range def.Abilities {
		for _, in := range a.Prog {
			if in.Op == kop.OpDraw {
				draws = append(draws, in)
			}
		}
	}
	if len(draws) != 2 {
		t.Fatalf("Cut a Deal: %d Draw instructions, want 2", len(draws))
	}
	if draws[0].C&kop.MvRemember == 0 || draws[0].C&kop.DrawRememberAll == 0 {
		t.Errorf("first Draw C %#x, want MvRemember|DrawRememberAll", draws[0].C)
	}
	if draws[1].A.K != kop.VExpr {
		t.Errorf("second Draw amount %+v, want an expression over the remembered count", draws[1].A)
	}
}
