package cardset_test

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHistoricPredicateUsesMatchupSubtypeIndex: PredHistoric's PredN is the
// pool's Saga subtype index+1, and a matchup renumbers subtypes into its own
// compact table. An unremapped PredN names some other subtype of the matchup,
// which an all-creature-types permanent (Mutavault) satisfies, so it would
// read as historic under On the Shoulders of Giants (CR 700.6: artifact,
// legendary or Saga only). The matchup's PredN must name its own Saga, or be
// 0 when the matchup has none.
func TestHistoricPredicateUsesMatchupSubtypeIndex(t *testing.T) {
	set := cardsettest.Synth(t, "std-trc")
	pt, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	saga := slices.Index(set.Subtypes(), "Saga")
	if saga < 0 {
		t.Fatal("the pool has no Saga subtype")
	}
	deck := []cardset.Entry{{Name: "Mutavault", Count: 1}, {Name: "On the Shoulders of Giants", Count: 1}}
	mt, _, _, err := set.Matchup(deck, nil)
	if err != nil {
		t.Fatal(err)
	}
	// precondition: the pool's Saga index is not the Saga slot here, so an
	// unremapped PredN names another subtype (or none)
	if saga < len(mt.Subtypes) && mt.Subtypes[saga] == "Saga" {
		t.Fatalf("the matchup keeps the pool's subtype numbering (Saga at %d)", saga)
	}
	checked := 0
	for i, pf := range pt.Filters {
		if pf.Pred != kop.PredHistoric || pf.PredN == 0 {
			continue
		}
		if int(pf.PredN) != saga+1 {
			t.Fatalf("pool filter %d: PredN %d, want the Saga index+1 %d", i, pf.PredN, saga+1)
		}
		for j, f := range mt.Filters {
			if f.Pred != kop.PredHistoric {
				continue
			}
			checked++
			if f.PredN != 0 && (int(f.PredN) > len(mt.Subtypes) || mt.Subtypes[f.PredN-1] != "Saga") {
				t.Errorf("matchup filter %d: PredN %d is not the matchup's Saga slot (subtypes %v)", j, f.PredN, mt.Subtypes)
			}
		}
		break
	}
	if checked == 0 {
		t.Fatal("no historic filter reached the matchup table")
	}
}
