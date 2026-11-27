package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDemoSacrificedCountShapes: Dargo, the Shipwrecker and Evendo Brushrazer
// lower with no refusal. Dargo's ReduceCost SVar is
// PlayerCountPropertyYou$SacrificedThisTurn Artifact,Creature and Evendo's
// Continuous CheckSVar is Permanent.!token, so ExSacrificedThisTurn carries a
// filter (X.F) for both; the union's clauses and the !token flag are the
// widened shape (no State field: the kernel reads the turn's zone-entry
// ledger).
func TestDemoSacrificedCountShapes(t *testing.T) {
	names := []string{"Dargo, the Shipwrecker", "Evendo Brushrazer"}
	lw := lowerNamedPool(t, names)
	if len(lw.cards) != len(names) {
		t.Fatalf("lowered %d cards, want %d", len(lw.cards), len(names))
	}
	for _, c := range lw.cards {
		for _, r := range c.Refusals {
			t.Errorf("%s refused: %s", c.Name, r.Reason)
		}
	}
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range bs {
		for _, r := range b.refusals {
			t.Errorf("%s build refusal: %v", b.def.Name, r)
		}
	}
	var filtered []kop.Expr
	for i := range pt.exprs {
		if e := &pt.exprs[i]; e.Kind == kop.ExSacrificedThisTurn && e.F != 0 {
			filtered = append(filtered, *e)
		}
	}
	if len(filtered) < 2 {
		t.Fatalf("%d filtered SacrificedThisTurn expressions, want one per card", len(filtered))
	}
	chainHas := func(f kop.FilterID, pred func(*kop.Filter) bool) bool {
		for ; f != 0; f = pt.filters[f].Or {
			if pred(&pt.filters[f]) {
				return true
			}
		}
		return false
	}
	var union, nontoken bool
	for i := range filtered {
		f := filtered[i].F
		if chainHas(f, func(kf *kop.Filter) bool { return kf.Types&kop.TArtifact != 0 }) &&
			chainHas(f, func(kf *kop.Filter) bool { return kf.Types&kop.TCreature != 0 }) {
			union = true
		}
		if chainHas(f, func(kf *kop.Filter) bool { return kf.Flags&kop.FNontoken != 0 }) {
			nontoken = true
		}
	}
	if !union {
		t.Error("no filtered expression carries the Artifact,Creature union")
	}
	if !nontoken {
		t.Error("no filtered expression carries Permanent.!token's nontoken flag")
	}
}
