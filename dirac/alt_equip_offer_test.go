package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// altEquipTable is an FDN Plains matchup with an Equipment whose equip costs
// {2}, a Kíli-shaped creature whose StatAltEquipCost static grants the equip
// a {0} alternative while the count gate (the zero count of the turn's first
// equip, ExThisTurnActExhaust N 1 EQ 0) holds, and a Bear to equip.
func altEquipTable(t *testing.T) (tbl *kop.Table, sword, kili, bear CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, _, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...),
		kop.Filter{},
		kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags: kop.FOther})
	nf := len(tbl.Filters)
	fAny, fEquipTgt := kop.FilterID(nf-2), kop.FilterID(nf-1)
	// the count gate: ExThisTurnActExhaust N 1 (the turn's Equip fact) EQ 0
	tbl.Exprs = append(tbl.Exprs, kop.Expr{Kind: kop.ExThisTurnActExhaust, N: 1})
	tbl.Conds = append(tbl.Conds, kop.Cond{Kind: kop.CondValue, Op: kop.CmpEQ,
		A: kop.ValueRef{K: kop.VExpr, N: int16(len(tbl.Exprs) - 1)}, B: kop.Const(0)})
	count0 := kop.CondID(len(tbl.Conds) - 1)
	sword = CardID(len(tbl.Cards))
	kili, bear = sword+1, sword+2
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Sword", Types: kop.TArtifact | kop.TEquipment,
			Abilities: []kop.Ability{
				{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostSorcery, Mana: kop.ManaCost{Generic: 2}}, AFlags: kop.AfEquip, NTargets: 1,
					Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fEquipTgt}},
					Prog:    []kop.Instr{{Op: kop.OpAttach, Obj: kop.RefTarget0}}},
			}},
		kop.CardDef{Name: "Test Kili", Types: kop.TCreature, Power: 1, Toughness: 2,
			Abilities: []kop.Ability{
				{Kind: kop.AbStatic, Cost: kop.Cost{},
					Static: kop.Static{Kind: kop.StatAltEquipCost, Filter: fAny, Cond2: count0}},
			}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	return tbl, sword, kili, bear
}

// equipOptions are the Sword's equip options in a priority decision: the
// printed cost's (Target 0) and the StatAltEquipCost static's alternative's
// (Target altEquipTgt).
func equipOptions(d Decision, sword Obj) (printed, alt Action, printedN, altN int) {
	for i := 0; i < int(d.N); i++ {
		a := d.Options[i]
		if a.Kind() != AkActivate || a.Source() != TCode(sword) {
			continue
		}
		switch a.Target() {
		case altEquipTgt:
			alt, altN = a, altN+1
		case 0:
			printed, printedN = a, printedN+1
		}
	}
	return
}

// TestAltEquipOffer: the StatAltEquipCost static's alternative (Kíli the
// Resourceful's) is a second equip option beside the printed cost, gated on
// the turn's first-equip count, priced at the alternative.
func TestAltEquipOffer(t *testing.T) {
	tbl, sword, _, _ := altEquipTable(t)
	type scene struct {
		sc                *scen
		sword, kili, bear Obj
	}
	setup := func(t *testing.T) scene {
		sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
		x := scene{sc: sc, sword: sc.perm(sword, 0), kili: sc.perm(sword+1, 0), bear: sc.perm(sword+2, 0)}
		sc.s.pl[0].pool[kop.C] = 2
		sc.s.ctl = ctlPriority
		return x
	}
	// offers reads the Sword's equip options off the priority decision.
	offers := func(t *testing.T, x scene) (printed, alt Action) {
		t.Helper()
		d := x.sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
		printed, alt, pn, an := equipOptions(d, x.sword)
		if pn != 1 || an != 1 {
			t.Fatalf("equip options: %d printed, %d alternative in %v", pn, an, d.Options[:d.N])
		}
		return printed, alt
	}
	// pickKili answers the equip's target ask with the Kíli creature.
	pickKili := func(t *testing.T, x scene) {
		t.Helper()
		d := x.sc.s.Step()
		if d.Kind != DkTarget {
			t.Fatalf("decision kind %d, want the equip's target ask", d.Kind)
		}
		for i := 0; i < int(d.N); i++ {
			if o := d.Options[i]; o.Target() == TCode(x.kili) {
				x.sc.s.Apply(o)
				return
			}
		}
		t.Fatalf("the Kíli creature not offered as the equip target: %v", d.Options[:d.N])
	}
	attachedToKili := func(x scene) bool {
		return x.sc.s.perms[x.sc.slot(x.sword)].attached == x.sc.slot(x.kili)
	}

	t.Run("alternative charges nothing", func(t *testing.T) {
		x := setup(t)
		_, alt := offers(t, x)
		x.sc.s.Apply(alt)
		pickKili(t, x)
		x.sc.resolveAll(t)
		if !attachedToKili(x) {
			t.Fatal("the Sword is not attached to the Kíli creature")
		}
		if got := x.sc.s.pl[0].pool[kop.C]; got != 2 {
			t.Fatalf("pool holds %d colorless, want 2 (the {0} alternative paid nothing)", got)
		}
	})
	t.Run("printed charges two", func(t *testing.T) {
		x := setup(t)
		printed, _ := offers(t, x)
		x.sc.s.Apply(printed)
		pickKili(t, x)
		x.sc.resolveAll(t)
		if got := x.sc.s.pl[0].pool[kop.C]; got != 0 {
			t.Fatalf("pool holds %d colorless, want 0 (the printed {2} paid)", got)
		}
	})
	t.Run("only printed after the turn's first equip", func(t *testing.T) {
		x := setup(t)
		_, alt := offers(t, x)
		x.sc.s.Apply(alt) // the {0} alternative: the pool keeps its {2}
		pickKili(t, x)
		x.sc.resolveAll(t)
		// the push recorded the activation (the count gate reads 1)
		d := x.sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
		_, _, pn, an := equipOptions(d, x.sword)
		if pn != 1 {
			t.Fatalf("printed equip not offered after the first: %v", d.Options[:d.N])
		}
		if an != 0 {
			t.Fatal("the alternative offered after the turn's first equip")
		}
		if got := x.sc.s.pl[0].pool[kop.C]; got != 2 {
			t.Fatalf("pool holds %d colorless, want 2", got)
		}
	})
	t.Run("mutation: DebugNoAltEquip drops the alternative only", func(t *testing.T) {
		DebugNoAltEquip = true
		defer func() { DebugNoAltEquip = false }()
		x := setup(t)
		d := x.sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
		if _, _, pn, an := equipOptions(d, x.sword); an != 0 || pn != 1 {
			t.Fatalf("DebugNoAltEquip: %d printed, %d alternative, want 1 printed and 0 alternative", pn, an)
		}
	})
}
