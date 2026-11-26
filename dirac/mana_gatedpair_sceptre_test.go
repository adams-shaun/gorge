package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// CheckGatedAnyPairOffer: Sceptre of Eternal Glory's second mana ability
// ({T}: three mana of any one colour, "activate only if you control three or
// more lands with the same name", a CheckSVar$ gate on Count$MostCardName,
// CR 602.5b) is part of the source's mana view only while the gate holds.
// Divination ({2}{U}) is castable from the Sceptre alone exactly then: the
// planner prices the gate-on pair's larger alternative (wideN), the gate-off
// view makes one mana. The lands are tapped so they pay nothing, three Plains
// open the gate, two Plains or three of the opponent's (or three differently
// named lands) do not.
func CheckGatedAnyPairOffer(t *testing.T, tbl *kop.Table, id map[string]CardID) {
	t.Helper()
	type landFn func(name string, n int, p uint8) []Obj
	cases := []struct {
		name  string
		build func(land landFn, sc *scen)
		gate  bool
	}{
		{"three own Plains", func(land landFn, sc *scen) { land("Plains", 3, 0) }, true},
		{"two own Plains", func(land landFn, sc *scen) { land("Plains", 2, 0) }, false},
		{"three opposing Plains", func(land landFn, sc *scen) { land("Plains", 3, 1) }, false},
		{"two Plains and a Forest", func(land landFn, sc *scen) { land("Plains", 2, 0); land("Forest", 1, 0) }, false},
		{"three Plains, one gone", func(land landFn, sc *scen) {
			sc.s.leave(sc.k, sc.slot(land("Plains", 3, 0)[0]), kop.ZoneGraveyard, false)
		}, false},
	}
	for _, cfg := range []Config{{NoElide: true}, {GorgeLowering: true, NoElide: true}, {GorgeAutopaySurface: true, NoElide: true}} {
		for _, c := range cases {
			sc := newScenTableLib(t, tbl, id["Plains"], cfg)
			land := func(name string, n int, p uint8) (os []Obj) {
				for i := 0; i < n; i++ {
					o := sc.perm(id[name], p)
					sc.s.perms[sc.slot(o)].flags |= pfTapped
					os = append(os, o)
				}
				return os
			}
			s := sc.perm(id["Sceptre of Eternal Glory"], 0)
			c.build(land, sc)
			div := sc.hand(id["Divination"], 0)
			pc := sc.s.manaPCGen(sc.k, sc.slot(s))
			if pc.manaAb < 0 || pc.manaN != 1 {
				t.Fatalf("%s: the ungated ability is not the view's ability 0 (manaAb %d, manaN %d)", c.name, pc.manaAb, pc.manaN)
			}
			if got := pc.wideN == 3 && pc.wideCol == kop.ColAny; got != c.gate {
				t.Errorf("cfg %+v, %s: gate-on view = %v (wideN %d), want %v", cfg, c.name, got, pc.wideN, c.gate)
			}
			if got := sc.castOffered(0, div); got != c.gate {
				t.Errorf("cfg %+v, %s: Divination offered from the Sceptre = %v, want %v", cfg, c.name, got, c.gate)
			}
		}
	}
}
