package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestReflAmtBesideAFilterOffersTheCastItsColourPays: Incubation Druid is the
// only untapped source (the Forest is tapped) beside a Boros Signet. Its
// reflected colours include the tapped Forest's {G}, so gorge's bound pays
// Llanowar Elves's {G} and its prefix search taps the Druid for {G}. The
// Signet's filter makes caBoard refuse the board, so scriptOffers searched
// the bound's plays only on ca (not caAny) and dropped the cast: tail-55 pair
// 45. The filter search takes the Druid as a step whose ask is its reflected
// candidates (reflAmtOther), the whole amount in one colour.
func TestReflAmtBesideAFilterOffersTheCastItsColourPays(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Llanowar Elves", Count: 1}, {Name: "Incubation Druid", Count: 1}, {Name: "Boros Signet", Count: 1},
		{Name: "Swamp", Count: 2}, {Name: "Forest", Count: 1},
	}
	tbl, da, _, err := edhSet(t).Matchup(deck, deck)
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
	forest := sc.perm(id["Forest"], 0)
	sc.s.perms[sc.slot(forest)].flags |= pfTapped
	druid := sc.perm(id["Incubation Druid"], 0)
	signet := sc.perm(id["Boros Signet"], 0)
	if !sc.k.isReflAmt(id["Incubation Druid"]) || !sc.s.caGaps(sc.k, 0).has(sc.slot(druid)) {
		t.Fatal("precondition: the Druid is not a live reflected Amount$ census gap")
	}
	if !sc.s.scriptFilt(sc.k, 0).has(sc.slot(signet)) {
		t.Fatal("precondition: the Signet is no filter census gap")
	}
	if sc.s.caBoard(sc.k, 0) || !sc.k.gen.anyComboAny {
		t.Fatal("precondition: the filter does not refuse the Combo Any board")
	}
	if !sc.castOffered(0, sc.hand(id["Llanowar Elves"], 0)) {
		t.Error("Llanowar Elves is not offered: the Druid's {G} beside the Signet was never searched")
	}
}
