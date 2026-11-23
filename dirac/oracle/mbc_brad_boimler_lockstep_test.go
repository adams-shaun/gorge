package oracle

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

const bradBoimlerRepro = "counter_order_entry_boimler_beside_doubler_unmodelled"

// Brad Boimler, Eager Ensign's tap trigger adds one to every counter put on a
// permanent its controller controls until end of turn (plusCtr). Beside a
// counter doubler (Dr. Joseph M'Benga) the two do not commute, so a creature's
// entry counters are a CR 616.1 order competition: gorge stages the entry and
// asks the order before the move (entry_counters.go stageEntryCounterOrder).
// The kernel poses its order ask only for a resolving OpPutCounter
// (repl_ctrorder.go ctrOrderPark); an entry placement (placeEntryCounters, from
// enterGen on every route: a resolving permanent spell, a land play, a token)
// would need a park with the card still in its zone on each of them, well
// past bounded dirac code, so the doubler beside the plus stalls replctr
// (std-trk pair 4). Brad Boimler is refused by name and its recorded game still
// stalls with it implicated.
func TestMBCBradBoimlerReprosLockstep(t *testing.T) {
	t.Parallel()
	entries, err := cardset.ReadRefused(os.DirFS(moduleRootFromOracle))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for i := range entries {
		e := &entries[i]
		if e.Card != "Brad Boimler, Eager Ensign" {
			continue
		}
		if e.Repro.Pool != "custom" || e.Repro.File != bradBoimlerRepro {
			t.Fatalf("Brad Boimler's entry replays %s/%s, want custom/%s", e.Repro.Pool, e.Repro.File, bradBoimlerRepro)
		}
		checkRefusedRepro(t, e)
		found = true
	}
	if !found {
		t.Fatal("Brad Boimler, Eager Ensign has no dirac/support/refused entry: beside a counter doubler on an entry placement gorge asks the CR 616.1 order and the kernel stalls replctr")
	}
}
