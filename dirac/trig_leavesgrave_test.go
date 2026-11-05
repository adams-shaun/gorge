package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGraveSweepToExileIsPerCard: gorge's effChangeZoneAll opens its zone
// batch only for a sweep onto the battlefield or off it, so a graveyard sweep
// to exile is not one batch: a "one or more cards leave your graveyard" line
// queues once per card (Tormod's Crypt beside Ketramose in lockstep,
// dirac/oracle TestOriginMultizoneLockstep), and the batch stays closed.
func TestGraveSweepToExileIsPerCard(t *testing.T) {
	sc, id := newLeftGraveyardScen(t, []cardset.Entry{
		{Name: "Chalk Outline", Count: 1}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Forest", Count: 4}, {Name: "Plains", Count: 8},
	})
	chalk, bears := id["Chalk Outline"], id["Grizzly Bears"]
	c := sc.perm(chalk, 0)
	ab := trigLeftgraveyardGraveAb(t, sc, chalk)
	sc.grave(bears, 0, 2)
	var f kop.FilterID
	for i, fl := range sc.k.t.Filters {
		if fl.Ctrl == kop.CtrlYou && fl.Zone == kop.ZoneGraveyard && fl.Types == 0 && fl.Or == 0 {
			f = kop.FilterID(i)
			break
		}
	}
	if f == 0 {
		t.Fatal("no graveyard filter in the pool")
	}
	in := kop.Instr{Op: kop.OpChangeZoneAll, C: int16(kop.ZoneExile), F: f}
	sc.s.npend = 0
	sc.s.res.e.ctrl = 0
	sc.s.execChangeZoneAll(sc.k, &in, false)
	if n := batchPend(sc, c, ab); n != 2 {
		t.Fatalf("a two-card sweep to exile queued %d triggers, want 2", n)
	}
	if sc.s.g.batchLatch != 0 {
		t.Fatalf("batch state left open: %#x", sc.s.g.batchLatch)
	}
}
