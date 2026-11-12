package cardset_test

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHistoricSagaMatchup: a matchup holding a Saga and a PredHistoric
// filter must keep the Saga subtype, remap PredN onto it and give the Saga
// card its mask bit (CR 700.6); the kernel reads exactly those two (predPermKind,
// predCardKind). tail-11 is used because std-trc keeps every printed subtype
// through the pool-wide choosesType escape and cannot witness the drop; the
// bucket index moves when a lowering changes the lowered tail's buckets.
func TestHistoricSagaMatchup(t *testing.T) {
	set := cardsettest.Synth(t, "tail-11")
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
	var hist []kop.Filter
	for _, f := range tbl.Filters {
		if f.Pred == kop.PredHistoric {
			hist = append(hist, f)
		}
	}
	if len(hist) == 0 {
		t.Fatal("the matchup has no PredHistoric filter")
	}
	for _, f := range hist {
		if f.PredN == 0 || int(f.PredN) > len(tbl.Subtypes) || tbl.Subtypes[f.PredN-1] != "Saga" {
			t.Fatalf("PredHistoric PredN %d does not name the matchup's Saga (subtypes %v)", f.PredN, tbl.Subtypes)
		}
	}
	c := tbl.Cards[ids[1]]
	if !c.Subtypes.Has(saga) {
		t.Fatalf("the Saga card's mask does not hold the Saga slot %d", saga)
	}
	if c.Types&(kop.TArtifact|kop.TLegendary) != 0 {
		t.Fatal("precondition: the Saga card is also an artifact or legendary")
	}
}
