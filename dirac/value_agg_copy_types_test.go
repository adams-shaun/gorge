package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestAggCardTypesReplacingCopyKeepsEarlierSlots: AggCardTypes unions each
// matching permanent's face card types (CR 205.2a), and a type-line
// replacement copy exception (Dino DNA's "creature", CR 707.9b) rewrites only
// its own slot's face. A replacing copy in a later slot than a Land must not
// erase the Land from the union, whatever the slot order.
func TestAggCardTypesReplacingCopyKeepsEarlierSlots(t *testing.T) {
	tbl := *v.Table
	tbl.Exprs = make([]kop.Expr, 1)
	tbl.Overrides = append([]kop.Override(nil), v.Table.Overrides...)
	tbl.Overrides = append(tbl.Overrides, kop.Override{HasTypes: true, SetTypes: kop.TCreature})
	idx := uint8(len(tbl.Overrides)) // fxCopyExc stores index+1
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "T Forest", Types: kop.TLand},
		kop.CardDef{Name: "T Aura", Types: kop.TEnchantment})
	forest, aura := n, n+1
	tbl.Filters = append(tbl.Filters, kop.Filter{Ctrl: kop.CtrlYou})
	f := kop.FilterID(len(tbl.Filters) - 1)

	for _, copyFirst := range []bool{false, true} {
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		sc.k.gen.cpxTy = true // no table ability references the appended override
		var cp Obj
		if copyFirst {
			cp = sc.perm(aura, 0)
			sc.perm(forest, 0)
		} else {
			sc.perm(forest, 0)
			cp = sc.perm(aura, 0)
		}
		sl := sc.slot(cp)
		sc.s.copyExcPush(sl, idx)
		if !sc.s.fxM.has(sl) {
			t.Fatal("precondition: the copy's slot has no exception record")
		}
		if got := sc.s.copyExcTy(sc.k, sl, kop.TEnchantment); got != kop.TCreature {
			t.Fatalf("precondition: the replacing copy reads types %b, want Creature only", got)
		}
		x := kop.Expr{Kind: kop.ExValidAgg, N: kop.AggCardTypes, F: f}
		got := sc.s.exprCold(sc.k, &x, evalCtx{ctrl: 0, src: NoObj, srcSl: NoSlot})
		if got != 2 {
			t.Errorf("copyFirst=%v: Land + replacing copy counts %d card types, want 2", copyFirst, got)
		}
	}
}
