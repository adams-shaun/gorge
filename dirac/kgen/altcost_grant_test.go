package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestAltCostGrantLowers: a battlefield grant of a mana alternative cost to
// the controller's spells (Fist of Suns, Jodah, Leyline of Mutation: any
// spell for {W}{U}{B}{R}{G}; Rooftop Storm: Zombie creature spells for {0};
// Runeforge Champion: Rune spells for {1}) lowers as a StatAlternativeCost
// carrying kop.StAltGrant, its Filter the ValidCard$ and its mana the price
// (gorge alternativeCosts' printed battlefield walk). It was refused as
// "AlternativeCost shape".
func TestAltCostGrantLowers(t *testing.T) {
	type want struct {
		filter bool
		mana   kop.ManaCost
	}
	wubrg := kop.ManaCost{Pips: [kop.NumManaKinds]uint8{1, 1, 1, 1, 1}}
	cases := map[string]want{
		"Fist of Suns":            {mana: wubrg},
		"Jodah, Archmage Eternal": {mana: wubrg},
		"Leyline of Mutation":     {mana: wubrg},
		"Rooftop Storm":           {filter: true},
		"Runeforge Champion":      {filter: true, mana: kop.ManaCost{Generic: 1}},
	}
	names := make([]string, 0, len(cases))
	for n := range cases {
		names = append(names, n)
	}
	all, bs := keclBuild(t, names)
	found := map[string]bool{}
	for i, c := range all {
		w, ok := cases[c.Name]
		if !ok {
			continue
		}
		found[c.Name] = true
		if r := lowerRefusals(c); len(r) != 0 {
			t.Errorf("%s refused: %v", c.Name, r)
			continue
		}
		n := 0
		for _, a := range bs[i].def.Abilities {
			if a.Kind != kop.AbStatic || a.Static.Kind != kop.StatAlternativeCost {
				continue
			}
			n++
			if a.Static.StFlags&kop.StAltGrant == 0 {
				t.Errorf("%s: grant flag unset (StFlags %b)", c.Name, a.Static.StFlags)
			}
			if got := a.Static.Filter != 0; got != w.filter {
				t.Errorf("%s: Filter set = %v, want %v", c.Name, got, w.filter)
			}
			if a.Cost.Mana != w.mana {
				t.Errorf("%s: mana %+v, want %+v", c.Name, a.Cost.Mana, w.mana)
			}
		}
		if n != 1 {
			t.Errorf("%s: %d StatAlternativeCost statics, want 1", c.Name, n)
		}
	}
	for n := range cases {
		if !found[n] {
			t.Errorf("%s: not lowered at all", n)
		}
	}
}
