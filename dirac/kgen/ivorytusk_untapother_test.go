package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestIvorytuskFortressBuilds: an UntapOtherPlayer static over the
// controller's creatures with a +1/+1 counter (Ivorytusk Fortress) lowers
// and builds as StatUntapOther, with no refusal at either level.
func TestIvorytuskFortressBuilds(t *testing.T) {
	lw := lowerNamedPool(t, []string{"Ivorytusk Fortress"})
	if len(lw.cards) != 1 || lw.cards[0].Name != "Ivorytusk Fortress" {
		t.Fatalf("pool %v, want Ivorytusk Fortress alone", lw.cards)
	}
	if rs := lw.cards[0].Refusals; len(rs) != 0 {
		t.Fatalf("lowering refused: %v", rs)
	}
	_, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	if rs := lw.cards[0].Refusals; len(rs) != 0 {
		t.Fatalf("build refused: %v", rs)
	}
	found := false
	for _, a := range bs[0].def.Abilities {
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatUntapOther {
			found = true
		}
	}
	if !found {
		t.Fatalf("no StatUntapOther built: %+v", bs[0].def.Abilities)
	}
}
