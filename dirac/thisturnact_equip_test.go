package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestThisTurnActEquipBit: ExThisTurnActExhaust N 1 (Count$ThisTurnActivated_
// Activated.Equip+YouCtrl) reads 0 until the controller activates an Equip
// ability, reads 1 after (the push records it, CR 602.2a), ignores another
// ability's activation, and reads 0 again once the turn changes.
func TestThisTurnActEquipBit(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...), kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou})
	fCreature := kop.FilterID(len(tbl.Filters) - 1)
	tbl.Exprs = append(append([]kop.Expr(nil), tbl.Exprs...), kop.Expr{Kind: kop.ExThisTurnActExhaust, N: 1})
	exprIx := int16(len(tbl.Exprs) - 1)
	sword := CardID(len(tbl.Cards))
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Sword", Types: kop.TArtifact | kop.TEquipment,
			Abilities: []kop.Ability{
				{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostSorcery}, AFlags: kop.AfEquip, NTargets: 1,
					Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fCreature}},
					Prog:    []kop.Instr{{Op: kop.OpAttach, Obj: kop.RefTarget0}}},
			}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
	if sc.k.gen == nil || !sc.k.gen.anyTurnF {
		t.Fatal("precondition: the Equip read does not arm the turn facts (gen.anyTurnF)")
	}
	sw, bear := sc.perm(sword, 0), sc.perm(sword+1, 0)
	if !sc.onBF(sw) || !sc.onBF(bear) {
		t.Fatal("precondition: the Sword and the Bear are on the battlefield")
	}
	sc.s.ctl = ctlPriority
	read := func() int {
		return sc.s.evalV(sc.k, kop.ValueRef{K: kop.VExpr, N: exprIx}, evalCtx{ctrl: 0})
	}
	if got := read(); got != 0 {
		t.Fatalf("before any equip activation the count reads %d, want 0", got)
	}
	d := sc.s.Step()
	var equip Action
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(sw)) {
			equip = a
		}
	}
	if equip == 0 {
		t.Fatal("precondition: the equip ability is not offered")
	}
	sc.s.Apply(equip)
	if d = sc.s.Step(); d.Kind != DkTarget {
		t.Fatalf("decision kind %d, want the equip's target ask", d.Kind)
	}
	sc.s.Apply(d.Options[0])
	if got := read(); got != 1 {
		t.Fatalf("after the equip push the count reads %d, want 1", got)
	}
	if got := sc.s.evalV(sc.k, kop.ValueRef{K: kop.VExpr, N: exprIx}, evalCtx{ctrl: 1}); got != 0 {
		t.Fatalf("the opponent's count reads %d, want 0", got)
	}
	sc.s.g.turnF = 0 // the turn change clears it (turn_gen.go)
	if got := read(); got != 0 {
		t.Fatalf("after the turn facts clear the count reads %d, want 0", got)
	}
}
