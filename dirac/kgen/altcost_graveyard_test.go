package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAltCostGraveyardSelfLowers: a card's own S:Mode$ AlternativeCost |
// ValidSA$ Spell.Self | EffectZone$ Graveyard static lowers, and the kernel
// offers the graveyard cast gorge offers (engine/rules
// legal_walk_grave_altcost.go): Raffine's Guidance's plain {2}{W}, Scourge of
// Nel Toth's {B}{B} beside Sac<2/Creature>, Squee, Dubious Monarch's {3}{R}
// beside ExileFromGrave<4/Card.Other> (the exile count rides kop.Cost.N, the
// mana beside it) and Worldheart Phoenix's {W}{U}{B}{R}{G} whose SpellCast
// trigger reads Count$AltCost.1.0 (kop.ExAltCost) and enters two +1/+1
// counters. The static's zone rides kop.Static.Zone (ZoneGraveyard), the one
// home the hand walk (offerAltFreeHand) and the grave walk
// (offerGraveyardAltCasts) read to keep the offer out of the hand.
func TestAltCostGraveyardSelfLowers(t *testing.T) {
	const wantZone = "want the static lowered with Zone ZoneGraveyard"
	all, bs, pt := keclBuildPT(t, []string{
		"Raffine's Guidance", "Scourge of Nel Toth", "Squee, Dubious Monarch", "Worldheart Phoenix",
	})
	find := func(name string) int {
		for k, c := range all {
			if c.Name == name {
				return k
			}
		}
		return -1
	}
	altAb := func(name string) *kop.Ability {
		j := find(name)
		if j < 0 {
			t.Fatalf("%s: not lowered at all", name)
		}
		if r := lowerRefusals(all[j]); len(r) != 0 {
			t.Fatalf("%s refused: %v", name, r)
		}
		for i := range bs[j].def.Abilities {
			a := &bs[j].def.Abilities[i]
			if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatAlternativeCost {
				return a
			}
		}
		t.Fatalf("%s: no StatAlternativeCost ability lowered", name)
		return nil
	}
	mana := func(w, u, b, r, g, gen uint8) kop.ManaCost {
		return kop.ManaCost{Generic: gen, Pips: [kop.NumManaKinds]uint8{w, u, b, r, g}}
	}
	type wish struct {
		name  string
		mana  kop.ManaCost
		flags kop.CostFlags
		n     uint8
		other bool // the exile filter names Card.Other
	}
	for _, w := range []wish{
		{"Raffine's Guidance", mana(1, 0, 0, 0, 0, 2), 0, 0, false},
		{"Scourge of Nel Toth", mana(0, 0, 2, 0, 0, 0), kop.CostSac, 2, false},
		{"Squee, Dubious Monarch", mana(0, 0, 0, 1, 0, 3), kop.CostExileGrave, 4, true},
		{"Worldheart Phoenix", mana(1, 1, 1, 1, 1, 0), 0, 0, false},
	} {
		st := altAb(w.name)
		if st.Static.Zone != kop.ZoneGraveyard {
			t.Errorf("%s: AlternativeCost static Zone=%d, %s", w.name, st.Static.Zone, wantZone)
		}
		if st.Cost.Mana != w.mana || st.Cost.Flags != w.flags || st.Cost.N != w.n {
			t.Errorf("%s: alternative cost mana %+v flags %d N %d, want %+v/%d/%d",
				w.name, st.Cost.Mana, st.Cost.Flags, st.Cost.N, w.mana, w.flags, w.n)
		}
		if w.other {
			if st.Cost.Filter == 0 || pt.filters[st.Cost.Filter].Flags&kop.FOther == 0 {
				t.Errorf("%s: exile filter id %d, want a Card.Other graveyard filter", w.name, st.Cost.Filter)
			}
		}
	}
	// Worldheart's SpellCast trigger reads Count$AltCost.1.0: the pool's expr
	// table carries an ExAltCost (the trigger's CheckSVar condition), and the
	// trigger's own body is the enters-with-two-counters Effect (the lowered
	// counter value is 2, the widened CounterNum$ spelling)
	if j := find("Worldheart Phoenix"); j >= 0 {
		alt := false
		for i := range pt.exprs {
			if pt.exprs[i].Kind == kop.ExAltCost {
				alt = true
			}
		}
		if !alt {
			t.Error("Worldheart Phoenix: no ExAltCost expr (Count$AltCost.1.0 not lowered)")
		}
	}
}
