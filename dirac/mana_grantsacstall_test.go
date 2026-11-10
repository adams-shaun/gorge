package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGrantPrefixSacrificeCompetingReplacementsStalls: a granted-mana prefix step on a
// Treasure's own sacrificing ability moves it at priority, outside the build's and a
// resolution's mana steps, so the move cannot park for gorge's CR 616.1 order ask.
// With two Festivals of Embers each exiling it instead, the move stalls by name
// (StallReplLeave, dieZone) and never picks an order gorge would ask for; with one
// Festival nothing competes and the Treasure is exiled without a stall.
func TestGrantPrefixSacrificeCompetingReplacementsStalls(t *testing.T) {
	tbl, err := cardsettest.Set(t, "blb").Pool()
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for c := range tbl.Cards {
		if _, ok := id[tbl.Cards[c].Name]; !ok {
			id[tbl.Cards[c].Name] = CardID(c)
		}
	}
	for _, name := range []string{"Clement, the Worrywort", "Festival of Embers", "c_a_treasure_sac"} {
		if _, ok := id[name]; !ok {
			t.Fatalf("%s not in the blb pool", name)
		}
	}
	for _, festivals := range []int{1, 2} {
		sc := newScenTable(t, tbl, Config{NoElide: true})
		sc.perm(id["Clement, the Worrywort"], 0)
		for i := 0; i < festivals; i++ {
			sc.perm(id["Festival of Embers"], 0)
		}
		tr := sc.perm(id["c_a_treasure_sac"], 0)
		sc.s.objs[tr].of |= ofToken
		s, k := sc.s, sc.k
		var g grantSrch
		g.k, g.p, g.np = k, 0, 1
		g.path[0] = grantStep{kind: stepAct, gi: -1, sl: sc.slot(tr), col: kop.G, n: 1}
		s.grantApply(k, &g)
		if z := s.objs[tr].zone; z == kop.ZoneBattlefield || z == kop.ZoneGraveyard {
			t.Errorf("%d Festivals: the Treasure is in zone %d, want exiled", festivals, z)
		}
		competes := festivals > 1
		if s.Stalled() != competes || competes && s.StallReason() != StallReplLeave {
			t.Errorf("%d Festivals: stalled %v reason %s, want stalled %v (StallReplLeave)",
				festivals, s.Stalled(), StallName(s.StallReason()), competes)
		}
	}
}
