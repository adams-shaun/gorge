package kgen

import (
	"os"
	"testing"
)

// The cmd5 pool (five Commander precons) builds as one table: the subtypes fit a
// table's masks and at least 302 of its cards lower.
func TestCmd5PoolBuilds(t *testing.T) {
	cards := tableCorpus(t)
	p, _, prev, err := readPool(os.DirFS("../.."), "cmd5", cards)
	if err != nil {
		t.Fatal(err)
	}
	if len(prev) == 0 {
		t.Fatal("cmd5 has no ids.tsv; run make dirac-tables")
	}
	d, cv, _, err := tableOf(p, "cmd5", prev)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("%d cards, %d lowered, %d refused, %d tokens", cv.cards, cv.lowered, len(cv.refused), d.NumTokens)
	if cv.cards != 321 {
		t.Errorf("%d cards, want 321", cv.cards)
	}
	if cv.lowered < 302 {
		t.Errorf("%d lowered, want at least 302", cv.lowered)
	}
}
