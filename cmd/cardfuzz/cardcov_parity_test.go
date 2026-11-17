package main

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/oracle"
	"github.com/adams-shaun/gorge/testkit/cardcov"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestKeysMatchCoverageKeys pins the two ports of the ability inventory
// together: testkit/cardcov.Keys (what cardfuzz and mtgsim credit) and
// dirac/oracle.CoverageKeys (the lockstep side) name the same keys, in the
// same order, for every sampled corpus card.
func TestKeysMatchCoverageKeys(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	all := reg.AllCards()
	if len(all) < 1000 {
		t.Fatalf("setup: corpus holds %d cards", len(all))
	}
	checked, withKeys := 0, 0
	for i, c := range all {
		if i%5 != 0 {
			continue
		}
		checked++
		got, want := cardcov.Keys(c), oracle.CoverageKeys(c)
		if !slices.Equal(got, want) {
			t.Errorf("%s: cardcov.Keys = %v, oracle.CoverageKeys = %v", c.Faces[0].Name, got, want)
		}
		if len(want) > 0 {
			withKeys++
		}
	}
	if withKeys < checked/10 {
		t.Fatalf("only %d of %d sampled cards carry keys: the sample proves nothing", withKeys, checked)
	}
}
