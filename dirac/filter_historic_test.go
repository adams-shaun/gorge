package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHistoricSagaMatchup: a matchup holding a Saga and a PredHistoric
// filter must keep the Saga subtype and remap PredN onto it (CR 700.6).
// tail-10 is used because std-trc keeps every printed subtype through the
// pool-wide choosesType escape and cannot witness the drop.
func TestHistoricSagaMatchup(t *testing.T) {
	set := cardsettest.Synth(t, "tail-10")
	deck := []cardset.Entry{{Name: "Abstergo Entertainment", Count: 1}, {Name: "Azusa's Many Journeys", Count: 1}}
	tbl, ids, _, err := set.Matchup(deck, nil)
	if err != nil {
		t.Fatal(err)
	}
	saga := -1
	for i, s := range tbl.Subtypes {
		if s == "Saga" {
			saga = i
		}
	}
	if saga < 0 {
		t.Fatalf("the matchup dropped the Saga subtype (subtypes %v)", tbl.Subtypes)
	}
	hist, found := kop.FilterID(0), false
	for i, f := range tbl.Filters {
		if f.Pred == kop.PredHistoric {
			hist, found = kop.FilterID(i), true
			break
		}
	}
	if !found {
		t.Fatal("the matchup has no PredHistoric filter")
	}
	if f := tbl.Filters[hist]; f.PredN == 0 || int(f.PredN) > len(tbl.Subtypes) || tbl.Subtypes[f.PredN-1] != "Saga" {
		t.Fatalf("PredHistoric PredN %d does not name the matchup's Saga (subtypes %v)", f.PredN, tbl.Subtypes)
	}
	sc := newScenTable(t, tbl, Config{NoElide: true})
	sagaID := ids[1]
	if !tbl.Cards[sagaID].Subtypes.Has(saga) {
		t.Fatalf("the Saga card's mask does not hold the Saga slot %d", saga)
	}
	if tbl.Cards[sagaID].Types&(kop.TArtifact|kop.TLegendary) != 0 {
		t.Fatal("precondition: the Saga card is also an artifact or legendary")
	}
	if !sc.s.matchPerm(sc.k, hist, sc.slot(sc.perm(sagaID, 0)), 0, NoSlot) {
		t.Error("a Saga permanent is not historic")
	}
	if !sc.s.matchCard(sc.k, hist, sagaID, 0, 0) {
		t.Error("a Saga card off the battlefield is not historic")
	}
}
