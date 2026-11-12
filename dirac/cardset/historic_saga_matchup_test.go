package cardset_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestHistoricSagaMatchup: a matchup holding a Saga and a PredHistoric
// filter must keep the Saga subtype, remap PredN onto it and give the Saga
// card its mask bit (CR 700.6); the kernel reads exactly those two (predPermKind,
// predCardKind). A tail pool is searched because std-trc keeps every printed
// subtype through the pool-wide choosesType escape and cannot witness the drop.
// No card or pool is named: TailBuckets hash-sorts every lowered name outside the
// Standard lists into fixed-size buckets, so each newly lowered card moves the
// boundaries and a pair that shared a bucket yesterday may not tomorrow. The
// witness is searched for: a Saga that is neither an artifact nor legendary (so
// only the Saga clause of the historic test can reach it) beside a card that
// reads Saga through a PredHistoric filter, both in one pool.
func TestHistoricSagaMatchup(t *testing.T) {
	deck, set := historicSagaWitness(t)
	t.Logf("witness in %s: %s beside the Saga %s", set.Name(), deck[0].Name, deck[1].Name)
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
		t.Fatalf("the matchup dropped the Saga subtype (pool %s deck %v subtypes %v)", set.Name(), deck, tbl.Subtypes)
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

// historicSagaWitness finds, in the first tail pool that has both, a plain Saga and
// a card that reads the Saga subtype through a PredHistoric filter, and returns the
// two-card deck {historic card, Saga} (the Saga is ids[1]) with the pool.
//
// A matchup table holds the whole pool's filters, so a PredHistoric filter alone
// proves nothing: only a card that reaches one keeps the Saga subtype in its own
// matchup. The historic card is therefore the one whose single-card matchup has a
// PredHistoric filter with PredN already set. A Saga is read off the pool table's
// subtype masks (Set.Def leaves CardDef.Subtypes empty), not off SagaN: a card with
// chapters but no Saga subtype cannot witness anything. It fails when the corpus is
// present and no tail pool holds the pair: the lowered cards no longer witness the
// case, and that must be seen, not skipped.
func historicSagaWitness(t *testing.T) ([]cardset.Entry, cardset.Set) {
	t.Helper()
	pools := 0
	for _, name := range cardsettest.SynthNames(t) {
		if !strings.HasPrefix(name, "tail-") {
			continue
		}
		pools++
		set := cardsettest.Synth(t, name)
		pool, err := set.Pool()
		if err != nil {
			t.Fatalf("pool %s: %v", name, err)
		}
		sagaSub := slices.Index(pool.Subtypes, "Saga")
		if sagaSub < 0 {
			continue
		}
		names := set.Names()
		saga := -1
		for i := 0; i < set.NumCards(); i++ {
			c := pool.Cards[set.PoolID(i)]
			if !set.Inert(i) && c.Subtypes.Has(sagaSub) && c.Types&(kop.TArtifact|kop.TLegendary) == 0 {
				saga = i
				break
			}
		}
		if saga < 0 {
			continue
		}
		for i := 0; i < set.NumCards(); i++ {
			if i == saga || set.Inert(i) {
				continue
			}
			tbl, _, _, err := set.Matchup([]cardset.Entry{{Name: names[i], Count: 1}}, nil)
			if err != nil {
				continue
			}
			for _, f := range tbl.Filters {
				if f.Pred == kop.PredHistoric && f.PredN > 0 {
					return []cardset.Entry{{Name: names[i], Count: 1}, {Name: names[saga], Count: 1}}, set
				}
			}
		}
	}
	t.Fatalf("none of %d tail pools holds a plain Saga and a card that reads Saga through a PredHistoric filter", pools)
	return nil, nil
}
