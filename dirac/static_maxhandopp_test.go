package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMaxHandOppKernel: a StatNoMaxHandSize carrying StHandOpp and StDynamic
// (Winter, Misanthropic Guide: seven minus the card types in its controller's
// graveyard, under delirium) binds the controller's opponents, not its
// controller, and only while Cond2 holds. With four types the opponent (seat
// 0, the active player) with six cards discards three; with three types the
// delirium gate fails and nothing is discarded; with five types the maximum
// is two. DebugHandOppAsYou (the flag read as "the controller") must diverge.
func TestMaxHandOppKernel(t *testing.T) {
	tbl := *v.Table
	tbl.Conds = append([]kop.Cond(nil), tbl.Conds...)
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondYourTurn}) // a generated table: k.gen != nil
	tbl.Exprs = append([]kop.Expr(nil), tbl.Exprs...)
	tbl.Exprs = append(tbl.Exprs,
		kop.Expr{Kind: kop.ExGraveTypes},
		kop.Expr{Kind: kop.ExGraveTypes, Add: -7, Neg: true}) // 7 - the card types
	types, seven := int16(len(tbl.Exprs)-2), int16(len(tbl.Exprs)-1)
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondValue, Op: kop.CmpGE,
		A: kop.ValueRef{K: kop.VExpr, N: types}, B: kop.Const(4)}) // delirium
	delirium := kop.CondID(len(tbl.Conds) - 1)
	n := CardID(len(tbl.Cards))
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Winter", Types: kop.TCreature, Power: 1, Toughness: 1, Abilities: []kop.Ability{
			{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatNoMaxHandSize, Cond2: delirium,
				StFlags: kop.StHandOpp | kop.StDynamic, VA: kop.ValueRef{K: kop.VExpr, N: seven}}}}},
		kop.CardDef{Name: "T Artifact", Types: kop.TArtifact},
		kop.CardDef{Name: "T Enchantment", Types: kop.TEnchantment})
	winter, art, ench := n, n+1, n+2
	yard := []CardID{v.Plains, v.GrizzlyBears, v.Shock, art, ench} // five types, in the order added

	run := func(nTypes, owner int, hand int) int {
		sc := newScenTable(t, &tbl, Config{NoElide: true})
		sc.perm(winter, uint8(owner))
		for _, c := range yard[:nTypes] {
			sc.grave(c, uint8(owner), 1)
		}
		if got := sc.s.condEval(sc.k, delirium, evalCtx{ctrl: uint8(owner), src: NoObj, srcSl: NoSlot}); got != (nTypes >= 4) {
			t.Fatalf("setup: %d types in the controller's graveyard, delirium %v", nTypes, got)
		}
		for i := 0; i < hand; i++ {
			sc.hand(v.Plains, 0)
		}
		sc.s.step = kop.StepCleanup
		sc.s.ctl = ctlStepBegin
		sc.s.Step()
		if sc.s.ctl == ctlCleanupDiscard {
			return int(sc.s.discardLeft)
		}
		return 0
	}
	for _, tc := range []struct {
		name         string
		types, owner int
		hand, want   int
	}{
		{"four types, opponent holds six", 4, 1, 6, 3},
		{"three types: no delirium", 3, 1, 6, 0},
		{"five types, opponent holds six", 5, 1, 6, 4},
		{"four types, opponent holds three", 4, 1, 3, 0},
		{"the active player controls it", 4, 0, 6, 0},
	} {
		if got := run(tc.types, tc.owner, tc.hand); got != tc.want {
			t.Errorf("%s: discardLeft %d, want %d", tc.name, got, tc.want)
		}
	}

	DebugHandOppAsYou = true
	defer func() { DebugHandOppAsYou = false }()
	if got := run(4, 1, 6); got != 0 {
		t.Errorf("mutation on: discardLeft %d, want 0 (the flag read as the controller binds nobody here)", got)
	}
	if got := run(4, 0, 6); got != 3 {
		t.Errorf("mutation on, the active player controls it: discardLeft %d, want 3", got)
	}
}
