package kgen

import (
	"os"
	"path/filepath"
	"testing"
)

// Every fixture pool commits both of its source files. A pool's cards.txt and
// ids.tsv are generated together; a commit carrying one without the other
// leaves a fresh checkout where make dirac-tables rewrites ids.tsv AFTER gorged
// was built, so the wire-dirac smoke server exits with "no card tables for this
// binary" and the browser gate mislabels it a bind race (11-26:
// grantedtoxicsanct, eight merge-queue batches). No corpus is needed: this is
// the committed tree, and queue-precheck runs ./dirac/kgen -short on every
// dirac/ change, before a batch forms.
func TestEveryFixturePoolCommitsItsIDs(t *testing.T) {
	pools, err := filepath.Glob("../cardset/pools/fixtures/*/cards.txt")
	if err != nil {
		t.Fatal(err)
	}
	if len(pools) == 0 {
		t.Fatal("no fixture pools found under ../cardset/pools/fixtures")
	}
	for _, cards := range pools {
		if _, err := os.Stat(filepath.Join(filepath.Dir(cards), idsFile)); err != nil {
			t.Errorf("%s has cards.txt but no committed %s: a fresh build rewrites it after gorged's tables are keyed", filepath.Dir(cards), idsFile)
		}
	}
}
