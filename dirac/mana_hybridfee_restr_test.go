package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHybridWitnessLentRestrictedManaPaysNoFee: the hybrid witness lends a
// creature-only restricted batch into the pool so the spell's remaining cost
// can use it (CR 106.6, 601.2g). That lent mana must not pay Castle
// Garenbrig's {2}{G}{G} activation fee: the fee is a land activation, not a
// creature spell, so only plain mana may fund it.
//
// Seat 0 has Castle Garenbrig, a Forest, a Plains and two Treasures, with one
// creature-only {G} floating. The hybrid creature costs {5}{G/W}: the floating
// {G} covers the hybrid pip, and the rest of the plain cost needs both
// Treasures beside the Castle's plain {G}. Castle's fee alternative {2}{G}{G}
// is short a green without the lent {G}, so pricing it as payable spends the
// restricted batch on a land activation. The fee plan carries the larger
// irreversible cost (key 1), so a planner that counts the lent mana runs it
// instead of the two-Treasure plan: the batch was spent on the fee and the
// pool holds none of the Plains' {W}.
func TestHybridWitnessLentRestrictedManaPaysNoFee(t *testing.T) {
	set := cardsettest.Set(t, "restrictedmana")
	tbl0, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	tbl := *tbl0
	tbl.Cards = append([]kop.CardDef(nil), tbl0.Cards...)
	tbl.Filters = append([]kop.Filter(nil), tbl0.Filters...)
	id := func(name string) CardID {
		i, ok := set.Lookup(name)
		if !ok {
			t.Fatalf("%s not in the restrictedmana table", name)
		}
		return set.PoolID(i)
	}
	castleID := id("Castle Garenbrig")
	var restr kop.FilterID
	for j := range tbl.Cards[castleID].Abilities {
		if a := &tbl.Cards[castleID].Abilities[j]; kop.RestrictedManaCostShape(a) {
			restr = a.ManaRestrict
			break
		}
	}
	if restr == 0 {
		t.Fatal("precondition: Castle Garenbrig has no restricted fee ability")
	}
	grove := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Grove", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap}, ManaColors: kop.ColG, ManaN: 1, ManaRestrict: restr},
	}})
	hyb := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Hybrid", Types: kop.TCreature, Power: 1, Toughness: 1,
		Cost: kop.ManaCost{Generic: 5, Hyb: [4]kop.ColorMask{kop.ColG | kop.ColW}}})
	treasure := CardID(len(tbl.Cards))
	tbl.Cards = append(tbl.Cards, kop.CardDef{Name: "Test Treasure", Types: kop.TArtifact, Abilities: []kop.Ability{
		{Kind: kop.AbMana, Cost: kop.Cost{Flags: kop.CostTap | kop.CostSacSelf, N: 1}, ManaColors: kop.ColAny, ManaN: 1},
	}})
	sc := newScenTableLib(t, &tbl, id("Plains"), Config{NoElide: true, GorgeLowering: true})
	s := sc.s
	sc.perm(id("Forest"), 0) // before the Castle: it enters untapped beside a Forest
	castle := sc.perm(castleID, 0)
	sc.perm(id("Plains"), 0)
	for i := 0; i < 2; i++ {
		sc.perm(treasure, 0)
	}
	groveObj := sc.perm(grove, 0)
	c := s.cardAt(sc.slot(groveObj))
	rab := &sc.k.t.Cards[c].Abilities[sc.k.gen.restr(c)]
	if !s.floatRestrAb(sc.k, 0, sc.slot(groveObj), rab, int(kop.G), 1) {
		t.Fatal("precondition: the Grove's {G} did not float restricted")
	}
	s.perms[sc.slot(groveObj)].flags |= pfTapped // the Grove is spent; only its floating mana remains
	hybObj := sc.hand(hyb, 0)
	if s.restrN(0) != 1 || s.restrOf(0, int(kop.G)) != 1 {
		t.Fatalf("precondition: restricted %v, want one creature-only {G}", s.rb[0])
	}
	*s.bld() = build{e: stackEntry{kind: seSpell, card: hyb, ctrl: 0, obj: hybObj}}
	s.ctl = ctlBuild
	s.lowerWitnessEx(sc.k, 0, NoObj)
	// The fee plan pays Castle's {2}{G}{G} from the lent {G}, the Forest's
	// {G} and the Plains' {W} plus a Treasure's mana, so its pool holds no
	// white. The no-fee plan pays {G} from the Castle's plain ability beside
	// the Forest, the Plains and both Treasures and leaves the Plains' {W}.
	if s.perms[sc.slot(castle)].flags&pfTapped == 0 {
		t.Fatal("precondition: neither Castle alternative was tapped")
	}
	if got := s.pl[0].pool; got[kop.W] < 1 {
		t.Fatalf("pool %v after the witness: the Plains' {W} was spent on Castle Garenbrig's fee with the creature-only batch (CR 106.6)", got)
	}
}

// TestPayPlanFeeLeavesLentRestrictedMana: the fee payment itself also keeps
// the hybrid witness's lent restricted units out of the fee's supply (CR
// 106.6). A fee of {2}{G}{G} is payable from two plain {G} and two {W}, but
// not from the one plain {G} beside one lent {G}: the lent unit may not pay
// the Castle's activation, and the failed payment leaves the pool untouched.
func TestPayPlanFeeLeavesLentRestrictedMana(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	s := sc.s
	fee := utype{fee: 2, feeCol: uint8(kop.ColG), feeN: 2}
	var claim [kop.NumManaKinds]int
	lent := [kop.NumManaKinds]uint8{kop.G: 1}
	s.pl[0].pool = [kop.NumManaKinds]uint8{kop.G: 2, kop.W: 2}
	if s.payPlanFee(sc.k, 0, &fee, &claim, false, lent) {
		t.Fatal("the fee was paid with the lent {G} standing in for a plain {G}")
	}
	if got := s.pl[0].pool; got != ([kop.NumManaKinds]uint8{kop.G: 2, kop.W: 2}) {
		t.Fatalf("pool %v after the failed fee, want it untouched", got)
	}
	s.pl[0].pool = [kop.NumManaKinds]uint8{kop.G: 3, kop.W: 2}
	if !s.payPlanFee(sc.k, 0, &fee, &claim, false, lent) {
		t.Fatal("two plain {G} and two {W} do not pay {2}{G}{G}")
	}
	if got := s.pl[0].pool; got != ([kop.NumManaKinds]uint8{kop.G: 1}) {
		t.Fatalf("pool %v after the fee, want only the lent {G} left", got)
	}
	if s.pl[0].pool[kop.G] != lent[kop.G] {
		t.Fatal("the fee spent the lent {G}")
	}
}
