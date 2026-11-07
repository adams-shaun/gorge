package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSacParkKeepsOneBatch: an effect sacrifices two of player 1's creatures
// while player 0 controls two Leylines of the Void, so each move is a CR 616.1
// order ask. Gorge keeps its departure batch open across the asks, so a "one
// or more creatures leave" line queues once for the flush, and each parked
// sacrifice is emitted only with its move, after its answer.
func TestSacParkKeepsOneBatch(t *testing.T) {
	sc, id := newSetScenTweak(t, "exileany", []cardset.Entry{
		{Name: "Leyline of the Void", Count: 2},
		{Name: "Midnight Reaper", Count: 1},
		{Name: "Grizzly Bears", Count: 2},
		{Name: "Plains", Count: 8},
	}, func(tbl *kop.Table, id map[string]CardID) {
		// Midnight Reaper's line as a "whenever one or more" leave line
		cd := &tbl.Cards[id["Midnight Reaper"]]
		cd.Abilities = append([]kop.Ability(nil), cd.Abilities...)
		for j := range cd.Abilities {
			if a := &cd.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDies {
				a.Trig.Event = kop.TrigLeaves
				a.Trig.Flags2 |= kop.Tf2Batch
				return
			}
		}
		t.Fatal("Midnight Reaper has no dies trigger to make a batch line")
	})
	s, k := sc.s, sc.k
	if !k.gen.anyBatch {
		t.Fatal("the table has no batch line")
	}
	sc.perm(id["Leyline of the Void"], 0)
	sc.perm(id["Leyline of the Void"], 0)
	reaper := sc.perm(id["Midnight Reaper"], 1)
	ab := batchAb(t, sc, id["Midnight Reaper"], kop.Tf2Batch)
	a, b := sc.perm(id["Grizzly Bears"], 1), sc.perm(id["Grizzly Bears"], 1)
	for _, o := range []Obj{a, b} {
		if !s.graveMoveCompetes(k, o) {
			t.Fatalf("object %d's move does not compete: no ask to park on", o)
		}
	}
	s.npend = 0
	s.addSac(sc.slot(a))
	s.addSac(sc.slot(b))
	s.flushSac(k)
	if s.Stalled() {
		t.Fatalf("the sacrifice stalled: %s", StallName(s.StallReason()))
	}
	if !s.parkedWaiting() || s.cold.parked[0].obj != a {
		t.Fatal("the first sacrifice is not parked for its ask")
	}
	if n := s.pl[1].sac[sacAll]; n != 0 {
		t.Fatalf("%d sacrifices emitted before the answer, want 0", n)
	}
	for asks := 0; s.parkedWaiting(); asks++ {
		if asks == 2 {
			t.Fatal("more than one ask per sacrificed creature")
		}
		var d Decision
		s.exileOrderOptions(k, &d)
		if d.Kind != DkChoose || d.Sub != ChGenReplOrder || d.N != 2 {
			t.Fatalf("decision kind %d sub %d n %d, want a two-source order ask", d.Kind, d.Sub, d.N)
		}
		s.applyExileOrder(k, d.Options[0])
	}
	for _, o := range []Obj{a, b} {
		if z := s.objs[o].zone; z != kop.ZoneExile {
			t.Fatalf("object %d is in zone %d, want exile", o, z)
		}
	}
	if n := s.pl[1].sac[sacAll]; n != 2 {
		t.Fatalf("%d sacrifices tallied, want 2", n)
	}
	if n := batchPend(sc, reaper, ab); n != 1 {
		t.Fatalf("one sacrifice flush queued %d leave-batch triggers, want 1", n)
	}
	if s.g.batchLatch != 0 {
		t.Fatalf("batch state left open: %#x", s.g.batchLatch)
	}
}
