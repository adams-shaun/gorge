package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDeadCopyTriggerTargetsWithTheCopiedColours: a blue creature that is a
// copy of a red one dies with the red card's "target creature you control" dies
// trigger. The trigger's source is the dead copy as it last was (red: CR 113.8,
// 608.2h), so a creature with protection from red is not a legal target, even
// though the entry's card (the copied one) is not the object's physical card.
func TestDeadCopyTriggerTargetsWithTheCopiedColours(t *testing.T) {
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	tbl, _, _, err := cardsettest.Set(t, "fdn").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(tbl.Filters, kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou}, kop.Filter{Types: kop.TCreature})
	mine, any := kop.FilterID(len(tbl.Filters)-2), kop.FilterID(len(tbl.Filters)-1)
	n := CardID(len(tbl.Cards))
	red, clone, bear, pro, destroy := n, n+1, n+2, n+3, n+4
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Red Pupil", Types: kop.TCreature, Colors: kop.ColR, Power: 1, Toughness: 1, Abilities: []kop.Ability{{Kind: kop.AbTriggered,
			Trig:    kop.Trigger{Event: kop.TrigDies, Flags: kop.TfSelf},
			Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: mine}}, NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpGainLife, Obj: kop.RefController, A: kop.Const(1)}}}}},
		kop.CardDef{Name: "Test Blue Clone", Types: kop.TCreature, Colors: kop.ColU, Power: 0, Toughness: 1},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2},
		kop.CardDef{Name: "Test Pro Red", Types: kop.TCreature, Power: 2, Toughness: 2, Keywords: kop.KwProtRed},
		kop.CardDef{Name: "Test Destroy", Types: kop.TSorcery, Abilities: []kop.Ability{{Kind: kop.AbSpell,
			Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: any}}, NTargets: 1,
			Prog: []kop.Instr{{Op: kop.OpDestroy, Obj: kop.RefTarget0}}}}})
	sc := newScenTable(t, tbl, Config{NoElide: true})
	cl := sc.perm(clone, 0)
	setCopy(sc, sc.slot(cl), red)
	sc.s.subscribe(sc.slot(cl), &sc.k.cards[red], true) // as the copy effect does (zones.go leave undoes it)
	bo, po := sc.perm(bear, 0), sc.perm(pro, 0)
	sc.do(t, AkCast, TCode(sc.hand(destroy, 0)), scAny)
	sc.do(t, AkTarget, scAny, TCode(cl))
	sc.do(t, AkPass, scAny, scAny)
	sc.do(t, AkPass, scAny, scAny)
	if sc.onBF(cl) {
		t.Fatal("the copy survived the destroy")
	}
	d := sc.s.Step()
	var gotBear, gotPro bool
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkTarget {
			gotBear = gotBear || a.Target() == TCode(bo)
			gotPro = gotPro || a.Target() == TCode(po)
		}
	}
	if !gotBear {
		t.Fatalf("the dies trigger's target decision does not offer the bear: kind %d opts %v", d.Kind, d.Options[:d.N])
	}
	if gotPro {
		t.Fatal("a creature with protection from red is offered as the red copy's dies trigger target")
	}
}
