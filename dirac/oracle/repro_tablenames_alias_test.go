package oracle

import (
	"errors"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestReproEntriesFindsAFlavourNamedCard: a deck lists a Universes-Within card
// by its flavour name (Baldin, Century Herdmaster), the pool's table names it
// so, and gorge's resolved card, which a repro records by, carries the real
// name (E. Honda, Sumo Champion): the replay answered `card "E. Honda, Sumo
// Champion" is not in the pool` for every pod that drew it (four-seat edh seed
// 3 pods 9 and 287, seed 4 pods 178 and 211).
func TestReproEntriesFindsAFlavourNamedCard(t *testing.T) {
	reg := synthReg(t)
	set, err := cardset.ByName("edh")
	if errors.Is(err, cardset.ErrNoTables) || errors.Is(err, cardset.ErrNoCorpus) {
		t.Skipf("no edh table (make dirac-tables): %v", err)
	}
	if err != nil {
		t.Fatal(err)
	}
	const flavour, real = "Baldin, Century Herdmaster", "E. Honda, Sumo Champion"
	if _, ok := set.Lookup(flavour); !ok {
		t.Skipf("the edh table has no %q", flavour)
	}
	if _, ok := set.Lookup(real); ok {
		t.Fatalf("precondition: the edh table names the card %q, not by its flavour name", real)
	}
	es := ReproEntries(reg, set, []string{real})
	if len(es) != 1 || es[0].Name != flavour {
		t.Fatalf("ReproEntries(%q) = %v, want the table's %q", real, es, flavour)
	}
}
