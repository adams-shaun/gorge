package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestReflAmtDruidBesideLeveledJoragaFundsTheCast: a counter-bearing Incubation
// Druid (three mana) beside a levelled Joraga Treespeaker (its granted
// "{T}: Add {G}{G}"), two Swamps and a Forest pay Silvos's {3}{G}{G}{G}
// (Druid {B}{B}{B}, Joraga {G}{G}, Forest {G}); gorge offers the cast.
func TestReflAmtDruidBesideLeveledJoragaFundsTheCast(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Silvos, Rogue Elemental", Count: 1}, {Name: "Incubation Druid", Count: 1}, {Name: "Joraga Treespeaker", Count: 1},
		{Name: "Swamp", Count: 2}, {Name: "Forest", Count: 1},
		{Name: "Bosk Banneret", Count: 1}, {Name: "A-Cauldron Familiar", Count: 1}, {Name: "Venerated Rotpriest", Count: 1}, {Name: "Pathbreaker Ibex", Count: 1}, {Name: "Nacatl Savage", Count: 1}, {Name: "Arctic Wolves", Count: 1}, {Name: "Boros Garrison", Count: 1}, {Name: "Ancient Brontodon", Count: 1},
	}
	tbl, da, _, err := cardsettest.Set(t, "reflamtleveler").Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	cfg := LockstepConfig(SurfaceFull)
	cfg.NoElide = true
	sc := newScenTableLib(t, tbl, id["Forest"], cfg)
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Forest"], 0)
	for _, n := range []string{"Bosk Banneret", "A-Cauldron Familiar", "Venerated Rotpriest", "Pathbreaker Ibex", "Nacatl Savage", "Arctic Wolves"} {
		sc.perm(id[n], 0)
	}
	gar := sc.perm(id["Boros Garrison"], 0)
	sc.s.perms[sc.slot(gar)].flags |= pfTapped
	druid := sc.perm(id["Incubation Druid"], 0)
	sc.s.perms[sc.slot(druid)].addCtrN(sc.s, int(kop.CtrP1P1), 3)
	jor := sc.perm(id["Joraga Treespeaker"], 0)
	sc.s.perms[sc.slot(jor)].addCtrN(sc.s, int(kop.CtrLevelUp), 1)
	if !sc.s.liveGrantGap(sc.k, 0) {
		t.Fatal("precondition: Joraga's level ability is no live granted mana ability")
	}
	if !sc.k.isReflAmt(id["Incubation Druid"]) || !sc.s.caGaps(sc.k, 0).has(sc.slot(druid)) {
		t.Fatal("precondition: the Druid is not a live reflected Amount$ census gap")
	}
	bront := sc.hand(id["Ancient Brontodon"], 0)
	if !sc.castOffered(0, bront) {
		t.Error("Brontodon not offered")
	}
	if !sc.castOffered(0, sc.hand(id["Silvos, Rogue Elemental"], 0)) {
		t.Error("Silvos is not offered: two Swamps, a Forest, a levelled Joraga and a counter-bearing Druid pay {3}{G}{G}{G}")
	}
}
