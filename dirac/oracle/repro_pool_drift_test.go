package oracle

import (
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// reproPoolDrift is the cards of r's decks its recorded synthetic pool no
// longer holds, sorted. Bucket membership of the tail pools is a function of the
// whole lowered-name list, so a lowering that changes its length moves one card
// per bucket boundary after the insertion point: a recording whose deck
// straddled the boundary then replays with `card "X" is not in the pool`, and
// only in the tests a run reaches. Pool names outside the synthetic namespaces
// (custom, standard-meta, one-off) are built from the recording itself and
// cannot drift.
func reproPoolDrift(t *testing.T, pool string, r *Repro) []string {
	t.Helper()
	set := cardsettest.Synth(t, pool)
	seen := map[string]bool{}
	var missing []string
	for _, d := range r.Cards {
		for _, n := range d {
			if seen[n] {
				continue
			}
			seen[n] = true
			if _, ok := set.Lookup(n); !ok {
				missing = append(missing, n)
			}
		}
	}
	sort.Strings(missing)
	return missing
}

// synthPoolName reports whether pool is a synthetic pool's name: its prefix is
// one of the synthetic namespaces and a table of it exists (the retired dr-mbc
// splits have recordings but no table, and no test replays them).
func synthPoolName(pool string, tables map[string]bool) bool {
	for _, p := range []string{"tail-", "std-", "dr-"} {
		if strings.HasPrefix(pool, p) {
			return tables[pool]
		}
	}
	return false
}

// TestReproDecksAreInTheirRecordedPool requires every recording on a synthetic
// pool to find all of its deck cards in that pool. The fix for a drifted
// recording is pool "custom" (reproSet): another synthetic pool cannot hold a
// deck whose cards span two buckets.
func TestReproDecksAreInTheirRecordedPool(t *testing.T) {
	t.Parallel()
	tables := map[string]bool{}
	for _, n := range cardsettest.SynthNames(t) {
		tables[n] = true
	}
	byPool := map[string][]string{}
	for _, path := range reproFiles(t) {
		r, err := ReadRepro(path)
		if err != nil {
			t.Fatalf("%s: %v", path, err)
		}
		if synthPoolName(r.Pool, tables) {
			byPool[r.Pool] = append(byPool[r.Pool], path)
		}
	}
	pools := make([]string, 0, len(byPool))
	for p := range byPool {
		pools = append(pools, p)
	}
	sort.Strings(pools)
	if len(pools) == 0 {
		t.Fatal("no recording names a synthetic pool")
	}
	for _, pool := range pools {
		for _, path := range byPool[pool] {
			r, err := ReadRepro(path)
			if err != nil {
				t.Fatalf("%s: %v", path, err)
			}
			if miss := reproPoolDrift(t, pool, r); len(miss) > 0 {
				t.Errorf("%s: pool %q no longer holds %q: set its pool to \"custom\"", reproName(path), pool, miss)
			}
		}
	}
}
