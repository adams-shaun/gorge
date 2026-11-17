package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckPhyrexianDualSourcePricedHalf is the kernel-side half of
// TestPhyrexianDualSourcePricedHalf (dirac_test): Hall of Tagsin's first mana
// ability ({T}: Add {C}) is a unit gorge's planner census prices, while its
// second ({1}, {T}: Add one mana of any color) is a census gap. Khenra
// Spellspear's transform costs {3}{U/P}: two Mountains and Hall's {C} pay the
// three generic, and the pip's two-life face pays the {U/P}. Excluding the
// whole source under the autopay surface would drop Hall's priced {C} too, so
// dirac would withhold the activation gorge's planner offers.
func CheckPhyrexianDualSourcePricedHalf(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{GorgeAutopaySurface: true, NoElide: true})
	khenra := sc.perm(id["Khenra Spellspear"], 0)
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Mountain"], 0)
	hall := sc.perm(id["Hall of Tagsin"], 0)
	hallSl := sc.slot(hall)

	// The test reads a dual source: the census ability is a priced unit and a
	// second mana ability is the census gap. Without both the assertion below
	// would be about some other shape.
	pc := sc.s.manaPCGen(sc.k, hallSl)
	cd := &sc.k.t.Cards[id["Hall of Tagsin"]]
	if pc.manaAb < 0 || int(pc.manaAb) >= len(cd.Abilities) {
		t.Fatalf("Hall's planner profile names no mana ability (%d)", pc.manaAb)
	}
	if censusGapMana(&cd.Abilities[pc.manaAb]) {
		t.Fatalf("Hall's census ability %d is itself a census gap: no priced half to keep", pc.manaAb)
	}
	gap := false
	for j := range cd.Abilities {
		if j != int(pc.manaAb) && cd.Abilities[j].Kind == kop.AbMana && censusGapMana(&cd.Abilities[j]) {
			gap = true
		}
	}
	if !gap {
		t.Fatal("Hall has no census-gap mana ability: the board is not a dual source")
	}

	ai := -1
	kcd := &sc.k.t.Cards[id["Khenra Spellspear"]]
	for j := range kcd.Abilities {
		a := &kcd.Abilities[j]
		if a.Kind == kop.AbActivated && a.Cost.Mana.Phyrexian() {
			ai = j
			break
		}
	}
	if ai < 0 {
		t.Fatal("Khenra Spellspear has no Phyrexian activation")
	}
	a := &kcd.Abilities[ai]
	cost := a.Cost.Mana
	kSl := sc.slot(khenra)

	availIn := func(mode uint8) avail {
		var av avail
		saved := sc.s.g.availX
		sc.s.g.availX = mode
		sc.s.availFor(sc.k, 0, &av)
		sc.s.g.availX = saved
		return av
	}
	// The plain priority walk and the gaps walk must both price Hall's {C}.
	for _, mode := range []uint8{0, availGaps} {
		av := availIn(mode)
		if !sc.s.phyActAffords(sc.k, kSl, a, 0, cost, &av) {
			t.Errorf("availX %d: the Phyrexian activation {3}{U/P} is withheld though two Mountains and Hall's priced {C} pay the generic and the two-life face pays the pip", mode)
		}
	}
}
