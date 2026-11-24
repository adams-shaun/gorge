package mtgish

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cardc/ir"
)

func TestTargetLowering(t *testing.T) {
	filtStubs(t)
	permF := &ir.Filter{Subject: ir.SubjPermanent, Zones: zoneBit(ir.ZoneBattlefield), Op: ir.FPred, Pred: ir.PAny}
	plrF := &ir.Filter{Subject: ir.SubjPlayer, Op: ir.FPred, Pred: ir.PAny}
	spellF := &ir.Filter{Subject: ir.SubjSpell, Zones: zoneBit(ir.ZoneStack), Op: ir.FPred, Pred: ir.PAny}
	gyF := &ir.Filter{Subject: ir.SubjCard, Zones: zoneBit(ir.ZoneGraveyard), Op: ir.FPred, Pred: ir.PAny}
	perms, plrs := tn("Permanents", "AnyPermanent"), tn("Players", "AnyPlayer")
	spells, gys := tn("Spells", "AnySpell"), tn("CardsInGraveyards", "AnyCardInAnyGraveyard")
	n2 := tn("TestOnly", "N", 2)
	x := tn("TestOnly", "X")
	xv := &ir.Value{Kind: ir.VX}
	cases := []struct {
		variant string
		args    []any
		stem    string
		want    ir.Target
	}{
		{"TargetPermanent", []any{perms}, "TargetPermanent", ir.Target{Min: 1, Max: 1, Filter: permF}},
		{"UptoOneTargetPermanent", []any{perms}, "TargetPermanent", ir.Target{Max: 1, Filter: permF}},
		{"TargetPlayer", []any{plrs}, "TargetPlayer", ir.Target{Min: 1, Max: 1, Filter: plrF}},
		{"TargetSpell", []any{spells}, "TargetSpell", ir.Target{Min: 1, Max: 1, Filter: spellF}},
		{"TargetGraveyardCard", []any{gys}, "TargetGraveyardCard", ir.Target{Min: 1, Max: 1, Filter: gyF}},
		{"UptoOneTargetGraveyardCard", []any{gys}, "TargetGraveyardCard", ir.Target{Max: 1, Filter: gyF}},
		{"NumberTargetPermanents", []any{n2, perms}, "TargetPermanents", ir.Target{Min: 2, Max: 2, Filter: permF}},
		{"NumberTargetPermanents", []any{x, perms}, "TargetPermanents", ir.Target{MaxV: xv, Filter: permF}},
		{"UptoNumberTargetPermanents", []any{n2, perms}, "TargetPermanents", ir.Target{Max: 2, Filter: permF}},
		{"UptoNumberTargetPermanents", []any{x, perms}, "TargetPermanents", ir.Target{MaxV: xv, Filter: permF}},
		{"AnyNumberOfTargetPermanents", []any{perms}, "TargetPermanents", ir.Target{Max: -1, Filter: permF}},
		{"UptoNumberTargetGraveyardCards", []any{n2, gys}, "TargetGraveyardCards", ir.Target{Max: 2, Filter: gyF}},
		{"TargetPlayerOrPermanent", []any{plrs, perms}, "TargetPlayerOrPermanent", ir.Target{Min: 1, Max: 1,
			Filter: &ir.Filter{Subject: ir.SubjObject, Op: ir.FAny, Args: []*ir.Filter{plrF, permF}}}},
		{"AnyTarget", nil, "AnyTarget", ir.Target{Min: 1, Max: 1, Filter: tgtAnyTarget()}},
	}
	for _, tc := range cases {
		c := &Ctx{}
		got, err := c.Target(tn("Target", tc.variant, tc.args...))
		if err != nil {
			t.Errorf("Target.%s: %v", tc.variant, err)
			continue
		}
		if !ir.Equal(got, &tc.want) {
			t.Errorf("Target.%s:\n got %+v\nwant %+v", tc.variant, *got, tc.want)
		}
		if slot, ok := c.TargetSlot("Ref_" + tc.stem); !ok || slot != 0 {
			t.Errorf("Target.%s did not declare %s: %d %v", tc.variant, tc.stem, slot, ok)
		}
	}

	// The CR 115.4 any-target filter.
	f := tgtAnyTarget()
	if f.Op != ir.FAny || len(f.Args) != 4 {
		t.Fatalf("any-target: %+v", f)
	}
	for i, w := range []string{"Creature", "Planeswalker", "Battle"} {
		a := f.Args[i]
		if a.Subject != ir.SubjPermanent || a.Zones != zoneBit(ir.ZoneBattlefield) || a.Pred != ir.PCardType || a.Word != w {
			t.Errorf("any-target arg %d: %+v", i, a)
		}
	}
	if p := f.Args[3]; p.Subject != ir.SubjPlayer || p.Pred != ir.PAny {
		t.Errorf("any-target player: %+v", p)
	}

	// Slots follow lowering order, and Ref_ variants then resolve.
	c := &Ctx{}
	for _, v := range []string{"TargetPermanent", "UptoOneTargetPermanent"} {
		if _, err := c.Target(tn("Target", v, perms)); err != nil {
			t.Fatal(err)
		}
	}
	if s, ok := c.TargetSlot("Ref_TargetPermanent1"); !ok || s != 0 {
		t.Errorf("Ref_TargetPermanent1 = %d %v", s, ok)
	}
	if s, ok := c.TargetSlot("Ref_TargetPermanent2"); !ok || s != 1 {
		t.Errorf("Ref_TargetPermanent2 = %d %v", s, ok)
	}
}
