package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTargetSpentSnapshot: a resolving spell counters its target spell and
// then reads that spell's cast spend through ExTgtSpent. The spend is
// snapshotted as the resolution starts, so it survives the counter that
// removes the target's stack entry. A spell cast for its full cost reads
// equal to its mana value and the condition fails, exactly as Unravel's
// "less than its mana value" gate.
func TestTargetSpentSnapshot(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	tbl.Exprs = append([]kop.Expr(nil), v.Table.Exprs...)
	tbl.Conds = append([]kop.Cond(nil), v.Table.Conds...)

	targetID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Cheap Spell", Types: kop.TInstant, Cost: kop.ManaCost{Generic: 3}})
	spentExpr := int16(len(tbl.Exprs))
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExTgtSpent})
	mvExpr := int16(len(tbl.Exprs))
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExMVOf, Ref: kop.RefTarget0})
	less := kop.CondID(len(tbl.Conds))
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondValue, Op: kop.CmpLT,
		A: kop.ValueRef{K: kop.VExpr, N: spentExpr}, B: kop.ValueRef{K: kop.VExpr, N: mvExpr}})
	resolvingID := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Unravel", Types: kop.TInstant, Abilities: []kop.Ability{{
		Kind: kop.AbSpell,
		Targets: [kop.MaxTargets]kop.TargetSpec{
			{Kind: kop.TgtSpell, Min: 1, Max: 1},
		},
		NTargets: 1,
		Prog: []kop.Instr{
			{Op: kop.OpCounter, Obj: kop.RefTarget0},
			{Op: kop.OpSkipUnless, C: int16(less), A: kop.Const(1)},
			{Op: kop.OpDraw, Obj: kop.RefController, A: kop.Const(1)},
		},
	}}})

	for _, c := range []struct {
		name string
		// spent is the pool mana recorded for the target spell; its printed
		// mana value is 3.
		spent int16
		draw  int
	}{
		{"less than its mana value", 1, 1},
		{"cast for its full cost", 3, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			sc := newScenTable(t, &tbl, Config{NoElide: true})
			s, k := sc.s, sc.k
			target := sc.obj(targetID, 1)
			s.objs[target].zone = kop.ZoneStack
			resolving := sc.obj(resolvingID, 0)
			s.objs[resolving].zone = kop.ZoneStack
			s.stack[0] = stackEntry{kind: seSpell, obj: target, card: targetID, ctrl: 1, trigAmt: c.spent}
			s.stack[1] = stackEntry{kind: seSpell, obj: resolving, card: resolvingID, ctrl: 0, nt: 1}
			s.stack[1].tgt[0] = uint8(target)
			s.sp = 2

			if got := s.castSpent(target); got != c.spent {
				t.Fatalf("precondition: the target entry records %d, want %d", got, c.spent)
			}
			if !s.targetStillLegal(k, &s.stack[1], 0) {
				t.Fatal("precondition: the target spell is not legal")
			}
			hand := s.pl[0].handN
			s.resolveTop(k)
			if s.objs[target].zone != kop.ZoneGraveyard {
				t.Fatalf("the counter did not move the target spell (zone %d)", s.objs[target].zone)
			}
			if got := s.castSpent(target); got != 0 {
				t.Fatalf("precondition: the countered spell still records %d", got)
			}
			if got := s.exprCold(k, &kop.Expr{Kind: kop.ExTgtSpent}, evalCtx{}); got != 0 {
				t.Fatalf("the resolution's snapshot survived it: %d", got)
			}
			wantHand := hand
			if c.draw != 0 {
				wantHand++
			}
			if got := s.pl[0].handN; got != wantHand {
				t.Fatalf("hand %d, want %d (the condition read the snapshot)", got, wantHand)
			}
			if s.g.tgtSpent != 0 {
				t.Fatalf("the snapshot was not cleared at the resolution end: %d", s.g.tgtSpent)
			}
		})
	}
}
