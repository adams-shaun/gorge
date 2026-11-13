package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// reduceEquipTable is an FDN Plains matchup with an Equipment whose equip
// costs {2}, a creature whose static lowers equip abilities that target it by
// {2} (Cloud, Planet's Champion's ValidTarget$ Card.Self shape), and a Bear.
func reduceEquipTable(t *testing.T) (tbl *kop.Table, sword, reducer, bear CardID) {
	t.Helper()
	deck := []cardset.Entry{{Name: "Plains", Count: 20}}
	fdnSet := cardsettest.Set(t, "fdn")
	tbl, _, _, err := fdnSet.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	tbl.Filters = append(append([]kop.Filter(nil), tbl.Filters...),
		kop.Filter{},
		kop.Filter{Flags: kop.FSelf},
		kop.Filter{Types: kop.TCreature, Ctrl: kop.CtrlYou, Flags: kop.FOther})
	nf := len(tbl.Filters)
	fAny, fSelf, fEquipTgt := kop.FilterID(nf-3), kop.FilterID(nf-2), kop.FilterID(nf-1)
	sword = CardID(len(tbl.Cards))
	reducer, bear = sword+1, sword+2
	tbl.Cards = append(append([]kop.CardDef(nil), tbl.Cards...),
		kop.CardDef{Name: "Test Sword", Types: kop.TArtifact | kop.TEquipment,
			Abilities: []kop.Ability{
				{Kind: kop.AbActivated, Cost: kop.Cost{Flags: kop.CostSorcery, Mana: kop.ManaCost{Generic: 2}}, AFlags: kop.AfEquip, NTargets: 1,
					Targets: [kop.MaxTargets]kop.TargetSpec{{Kind: kop.TgtObject, Filter: fEquipTgt}},
					Prog:    []kop.Instr{{Op: kop.OpAttach, Obj: kop.RefTarget0}}},
			}},
		kop.CardDef{Name: "Test Reducer", Types: kop.TCreature, Power: 1, Toughness: 1,
			Abilities: []kop.Ability{
				{Kind: kop.AbStatic, Static: kop.Static{Kind: kop.StatReduceAbility, Filter: fAny, Filter2: fSelf,
					A: 2, B: kop.ReduceAbEquip, StFlags: kop.StTargetsFilter2}},
			}},
		kop.CardDef{Name: "Test Bear", Types: kop.TCreature, Power: 2, Toughness: 2})
	return tbl, sword, reducer, bear
}

// TestReduceCostEquipTargetSelf: "equip abilities you activate that target
// this creature cost {2} less" lowers an equip by the target it names: the
// offer prices the cheapest target, the target menu keeps only the payable
// candidates, and the charge prices the chosen target (CR 601.2c, 601.2f).
func TestReduceCostEquipTargetSelf(t *testing.T) {
	tbl, swordID, redID, bearID := reduceEquipTable(t)
	if tbl.Cards[swordID].Abilities[0].Cost.Mana.Generic != 2 || tbl.Cards[redID].Abilities[0].Static.A != 2 {
		t.Fatal("precondition: the equip costs {2} and the reduction is {2}")
	}
	type scene struct {
		sc                   *scen
		sword, reducer, bear Obj
	}
	setup := func(t *testing.T, pool uint8) scene {
		sc := newScenTableLib(t, tbl, 0, Config{NoElide: true})
		sw, r, b := sc.perm(swordID, 0), sc.perm(redID, 0), sc.perm(bearID, 0)
		if !sc.onBF(sw) || !sc.onBF(r) || !sc.onBF(b) {
			t.Fatal("precondition: the Sword and both creatures are on the battlefield")
		}
		sc.s.pl[0].pool[kop.C] = pool
		sc.s.ctl = ctlPriority
		return scene{sc, sw, r, b}
	}
	// equipOffered applies the Sword's equip if offered
	equipOffered := func(t *testing.T, x scene) bool {
		d := x.sc.s.Step()
		if d.Kind != DkPriority {
			t.Fatalf("decision kind %d, want priority", d.Kind)
		}
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(x.sword)) {
				x.sc.s.Apply(a)
				return true
			}
		}
		return false
	}
	// pick answers the target ask with tgt, after checking the menu
	pick := func(t *testing.T, x scene, tgt Obj, wantN int) {
		d := x.sc.s.Step()
		if d.Kind != DkTarget {
			t.Fatalf("decision kind %d, want the equip's target ask", d.Kind)
		}
		if wantN > 0 && int(d.N) != wantN {
			t.Fatalf("target ask has %d options %v, want %d", d.N, d.Options[:d.N], wantN)
		}
		for i := 0; i < int(d.N); i++ {
			if d.Options[i].Target() == TCode(uint8(tgt)) {
				x.sc.s.Apply(d.Options[i])
				return
			}
		}
		t.Fatalf("target %d not offered: %v", tgt, d.Options[:d.N])
	}
	attachedTo := func(x scene, o Obj) bool {
		return x.sc.s.perms[x.sc.slot(x.sword)].attached == x.sc.slot(o)
	}

	t.Run("targets the reducer: free", func(t *testing.T) {
		x := setup(t, 2)
		if !equipOffered(t, x) {
			t.Fatal("equip not offered with {2} floating")
		}
		pick(t, x, x.reducer, 2)
		x.sc.resolveAll(t)
		if !attachedTo(x, x.reducer) {
			t.Fatal("the Sword is not attached to the Reducer")
		}
		if got := x.sc.s.pl[0].pool[kop.C]; got != 2 {
			t.Fatalf("pool holds %d colorless, want 2 (the equip cost {0})", got)
		}
	})
	t.Run("targets the bear: full price", func(t *testing.T) {
		x := setup(t, 2)
		if !equipOffered(t, x) {
			t.Fatal("equip not offered with {2} floating")
		}
		pick(t, x, x.bear, 2)
		x.sc.resolveAll(t)
		if !attachedTo(x, x.bear) {
			t.Fatal("the Sword is not attached to the Bear")
		}
		if got := x.sc.s.pl[0].pool[kop.C]; got != 0 {
			t.Fatalf("pool holds %d colorless, want 0 (the equip cost {2})", got)
		}
	})
	t.Run("no mana: offered, only the reducer", func(t *testing.T) {
		x := setup(t, 0)
		if !equipOffered(t, x) {
			t.Fatal("equip not offered with no mana (the Reducer target makes it free)")
		}
		pick(t, x, x.reducer, 1)
		x.sc.resolveAll(t)
		if !attachedTo(x, x.reducer) {
			t.Fatal("the Sword is not attached to the Reducer")
		}
	})
	t.Run("mutation: no reduction, no offer", func(t *testing.T) {
		DebugNoAbReduce = true
		defer func() { DebugNoAbReduce = false }()
		x := setup(t, 0)
		if equipOffered(t, x) {
			t.Fatal("equip offered with no mana and the reduction dropped")
		}
	})
}
