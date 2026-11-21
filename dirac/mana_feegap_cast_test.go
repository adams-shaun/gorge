package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// manaFeeGapCastScen deals the divergence board (pool tail-73): p0's
// battlefield is nMountain Mountains, nForest Forests, the Shadowblood Ridge
// ("{1}, {T}: Add {B}{R}") when withRidge and, when withUntaidake, Untaidake,
// the Cloud Keeper ("{T}, Pay 2 life: Add {C}{C}. Spend this mana only to
// cast legendary spells"), untapped when untapped; Geosurge ({R}{R}{R}{R})
// sits in hand and p0's life is 5.
func manaFeeGapCastScen(t *testing.T, nMountain, nForest int, withRidge, withUntaidake, untapped bool) (*scen, Obj) {
	t.Helper()
	set := cardsettest.Synth(t, "tail-73")
	for _, n := range []string{"Geosurge", "Shadowblood Ridge", "Untaidake, the Cloud Keeper", "Mountain", "Forest"} {
		if _, ok := set.Lookup(n); !ok {
			t.Fatalf("%s not in the tail-73 pool", n)
		}
	}
	deck := []cardset.Entry{{Name: "Geosurge", Count: 1}, {Name: "Shadowblood Ridge", Count: 1},
		{Name: "Untaidake, the Cloud Keeper", Count: 1}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2}}
	tbl, da, _, err := set.Matchup(deck, []cardset.Entry{{Name: "Forest", Count: 20}})
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Forest"], Config{NoElide: true})
	for i := 0; i < nMountain; i++ {
		sc.perm(id["Mountain"], 0)
	}
	for i := 0; i < nForest; i++ {
		sc.perm(id["Forest"], 0)
	}
	if withRidge {
		sc.perm(id["Shadowblood Ridge"], 0)
	}
	var untaidake Obj
	if withUntaidake {
		if untapped {
			untaidake = sc.untappedPerm(id["Untaidake, the Cloud Keeper"], 0)
		} else {
			untaidake = sc.perm(id["Untaidake, the Cloud Keeper"], 0)
		}
	}
	g := sc.hand(id["Geosurge"], 0)
	sc.s.pl[0].life = 5
	if withUntaidake && untapped {
		if sc.s.perms[sc.slot(untaidake)].flags&pfTapped != 0 {
			t.Fatal("Untaidake was dealt tapped")
		}
		// the fix's precondition: the restricted "{T}, Pay 2 life" ability is
		// a planner last-resort unit, and it is no census gap (gfGapOther).
		c := id["Untaidake, the Cloud Keeper"]
		cd := &sc.k.t.Cards[c]
		restrLife := false
		for j := range cd.Abilities {
			if restrLifeAb(&cd.Abilities[j]) {
				restrLife = true
				break
			}
		}
		if !restrLife {
			t.Error("Untaidake's ability does not classify as a restricted life unit")
		}
		if sc.k.gen.has(c, gfGapOther) {
			t.Error("Untaidake is registered as a census gap (gfGapOther)")
		}
	}
	return sc, g
}

// manaFeeGapCastOffered walks the scenario's priority options and reports
// whether a cast of o is among them.
func manaFeeGapCastOffered(sc *scen, o Obj) bool {
	var d Decision
	sc.s.priorityOptions(sc.k, &d)
	return castOffered(&d, o)
}

// TestManaFeeGapFilterCastOffered: a fee-carrying filter (Shadowblood Ridge)
// beside a restricted "{T}, Pay N life" source (Untaidake, the Cloud Keeper)
// is still a filter-only board. The restricted life unit is a planner
// last-resort unit like a sacrifice-only source (mana_restrictedcost.go), so
// it keeps no filter off the filter search: the ridge's fee-carrying
// production funds the cast of Geosurge ({R}{R}{R}{R}) from 3 Mountains and
// 2 Forests (one mana pays the ridge's {1}, the ridge makes the fourth {R};
// CR 605.3a, 601.2g). gorge's one prefix search offers the cast; the kernel
// did not, because Untaidake held the board off filtOnly (gapOthers) while
// mixBoard had no component to model its production.
func TestManaFeeGapFilterCastOffered(t *testing.T) {
	sc, g := manaFeeGapCastScen(t, 3, 2, true, true, true)
	if !manaFeeGapCastOffered(sc, g) {
		t.Errorf("ridge beside an untapped Untaidake at life 5: Geosurge cast not offered (gorge offers it)")
	}
	// Untapped Untaidake, no ridge: its restricted {C}{C} may pay no
	// non-legendary spell, so neither engine offers the cast (the restriction
	// holds; the restricted life unit is no plain census unit).
	sc, g = manaFeeGapCastScen(t, 3, 2, false, true, true)
	if manaFeeGapCastOffered(sc, g) {
		t.Errorf("no ridge: Geosurge cast offered although Untaidake's mana may pay only legendary spells")
	}
	// Untaidake tapped (gapMask drops it) and no Untaidake at all: the board
	// is filter-only either way, and the ridge alone funds the cast.
	sc, g = manaFeeGapCastScen(t, 3, 2, true, true, false)
	if !manaFeeGapCastOffered(sc, g) {
		t.Errorf("ridge beside a tapped Untaidake: Geosurge cast not offered")
	}
	sc, g = manaFeeGapCastScen(t, 3, 2, true, false, false)
	if !manaFeeGapCastOffered(sc, g) {
		t.Errorf("ridge alone: Geosurge cast not offered")
	}
}
