package oracle

import (
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLockstepSetSeedsMatchSerial pins lockstepSet's reporting: the seeds its
// parallel chunks find divergent are the seeds a serial replay of each seed
// finds, and the failure line it ends with names every one of them, ascending.
// The pool is the phyact pool with the kernel's Savannah Lions made a 3/1, so
// the seeds that cast a Lions diverge, across three chunks of lockstepChunk.
// The divergence is planted: a test that leans on a live kernel defect goes
// stale when the defect is fixed, as this one did once phyact played clean.
func TestLockstepSetSeedsMatchSerial(t *testing.T) {
	const seeds = 130
	reg, set, gd := lockstepPool(t, "phyact", kphyactDecks)
	mutate := func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			if cards[i].Name == "Savannah Lions" {
				cards[i].Power = 3
			}
		}
		tbl.Cards = cards
	}
	var serial []uint64
	for s := uint64(1); s <= seeds; s++ {
		_, _, d := lockstepSeedsMode(t, reg, set, "phyact", kphyactDecks, gd, s, s, lockstepQuiet, mutate, nil, castFirstPolicy)
		serial = append(serial, divSeedsOf(d)...)
	}
	if len(serial) < 2 || serial[len(serial)-1] <= 2*lockstepChunk {
		t.Fatalf("serial replay of seeds 1..%d found %v divergent: the pool no longer exercises the check", seeds, serial)
	}
	var par []lockstepDivergence
	seedChunks(t, 1, seeds, lockstepChunk, func(t *testing.T, lo, hi uint64, mu *sync.Mutex) {
		_, _, d := lockstepSeedsMode(t, reg, set, "phyact", kphyactDecks, gd, lo, hi, lockstepQuiet, mutate, nil, castFirstPolicy)
		mu.Lock()
		par = append(par, d...)
		mu.Unlock()
	})
	// the chunks finish in any order: the report is the same for any of them
	rev := slices.Clone(par)
	slices.Reverse(rev)
	report := divergenceReport("phyact", seeds, rev)
	if got := divSeedsOf(par); !slices.Equal(slices.Sorted(slices.Values(got)), serial) {
		t.Fatalf("parallel chunks found %v divergent, the serial replay %v", got, serial)
	}
	if want := fmt.Sprintf("phyact: %d of %d seeds diverge: %v\n", len(serial), seeds, serial); !strings.HasPrefix(report, want) {
		t.Errorf("report %q does not start with the whole failing set %q", report, want)
	}
	if report != divergenceReport("phyact", seeds, par) {
		t.Errorf("the report depends on the order the chunks finish in")
	}
}

func divSeedsOf(ds []lockstepDivergence) []uint64 {
	var out []uint64
	for _, d := range ds {
		out = append(out, d.seed)
	}
	return out
}
