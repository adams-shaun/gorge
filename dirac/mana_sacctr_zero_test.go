package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// A counter-scaled sacrifice source with no counters resolves its amount to
// zero, so gorge's census drops it (payment_window_yield.go's
// manaAbilityYieldsNothing): it can contribute no payment, and no prefix
// activates it. The kernel's script must do the same. Listing it would let a
// prefix tap it (and, here, sacrifice it for nothing) even though the cast is
// already payable, which is a real Sacrificed event gorge never emits and
// leaves the boards different for every later decision.
func TestSacCtrZeroCounterSourcePaysNothing(t *testing.T) {
	tbl := *v.Table
	tbl.Cards = append([]kop.CardDef(nil), v.Table.Cards...)
	shrine := CardID(len(tbl.Cards))
	wall := shrine + 1
	poacher := wall + 1
	tbl.Cards = append(tbl.Cards,
		kop.CardDef{Name: "Test Shrine", Types: kop.TArtifact, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostTap | kop.CostSacSelf},
			AFlags:     kop.AfManaDynamic,
			ManaV:      kop.ValueRef{K: kop.VCountersSelf, N: int16(kop.CtrCharge)},
			ManaColors: kop.ColC,
		}}},
		kop.CardDef{Name: "Test Wall", Types: kop.TCreature, Power: 0, Toughness: 5, Abilities: []kop.Ability{{
			Kind:       kop.AbMana,
			Cost:       kop.Cost{Flags: kop.CostAddCtr | kop.CostOncePerTurn, Ctr: kop.CtrM0M1, N: 1},
			ManaColors: kop.ColG,
		}}},
		kop.CardDef{Name: "Test Poacher", Cost: kop.ManaCost{Generic: 3, Pips: [kop.NumManaKinds]uint8{kop.B: 1}},
			Types: kop.TCreature, Power: 2, Toughness: 4},
	)
	sc := newScenTable(t, &tbl, Config{NoElide: true})
	shrineObj := sc.perm(shrine, 0)
	sc.perm(wall, 0)
	for i := 0; i < 3; i++ {
		sc.perm(v.Swamp, 0)
	}
	poacherObj := sc.hand(poacher, 0)
	sl := sc.slot(shrineObj)
	if n := sc.s.perms[sl].ctrN(sc.s, int(kop.CtrCharge)); n != 0 {
		t.Fatalf("precondition: the Shrine holds %d charge counters, want 0", n)
	}
	if sc.s.scriptCtr(sc.k, 0).has(sl) {
		t.Error("the zero-counter Shrine is listed as a counter-cost gap")
	}
	// The {3}{B} cast is priced through the Wall's gap and the three Swamps;
	// the prefix must not touch the empty Shrine.
	sc.s.scriptPrefix(sc.k, MakeAction(AkCast, TCode(poacherObj), 0, 0), 0)
	if z := sc.s.objs[shrineObj].zone; z != kop.ZoneBattlefield {
		t.Fatalf("the prefix sacrificed the zero-counter Shrine (zone %d)", z)
	}
	if sc.s.perms[sl].flags&pfTapped != 0 {
		t.Fatal("the prefix tapped the zero-counter Shrine")
	}
	// The guard is the resolved amount, not the shape: a charged Shrine is
	// still a live gap.
	sc.s.perms[sl].addCtrN(sc.s, int(kop.CtrCharge), 1)
	if !sc.s.scriptCtr(sc.k, 0).has(sl) {
		t.Fatal("the charged Shrine is not listed as a counter-cost gap")
	}
}
