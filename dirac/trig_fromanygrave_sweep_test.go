package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// sweepPendingBy counts the queued triggers whose source is o. A split's
// second half queues under the first half's printed identity (SplitPrev), so
// this counts by source rather than by ability index.
func sweepPendingBy(sc *scen, o Obj) (n int) {
	for i := 0; i < int(sc.s.npend); i++ {
		if sc.s.pend[i].obj == o {
			n++
		}
	}
	return n
}

// TestFromAnyGraveSweepSilentWithoutBatch pins the non-batched leaveAll arm:
// a permanent whose printed "put into a graveyard from anywhere" line is a
// split Dies half must not fire that half on a later member's move of the same
// simultaneous departure. The batched arm marks each member pfBatchGone; the
// non-batched arm (no Tf2Batch line in the table, or the commander prefix) did
// not, so the departed source's half fired (Profane Memento after Boompile's
// destroy-all, the pair-57 divergence).
func TestFromAnyGraveSweepSilentWithoutBatch(t *testing.T) {
	sc, id := newSetScenTweak(t, "mbc", []cardset.Entry{
		{Name: "Compost", Count: 2},
		{Name: "Typhoid Rats", Count: 2},
		{Name: "Plains", Count: 8},
	}, nil)
	s, k := sc.s, sc.k
	if k.gen == nil || k.gen.anyBatch {
		t.Fatalf("precondition: gen %v anyBatch %v; this table must exercise the non-batched leaveAll arm", k.gen != nil, k.gen != nil && k.gen.anyBatch)
	}
	leaving := sc.perm(id["Compost"], 0)
	staying := sc.perm(id["Compost"], 0)
	rats := sc.perm(id["Typhoid Rats"], 1)
	// The gate is on the Dies half of a printed from-anywhere grave line:
	// Compost's "whenever a card is put into an opponent's graveyard from
	// anywhere" (lower_anytograve.go).
	found := false
	for i := range k.t.Cards[id["Compost"]].Abilities {
		a := &k.t.Cards[id["Compost"]].Abilities[i]
		if a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDies {
			if !a.FromAnyGrave {
				t.Fatal("precondition: Compost's Dies half does not carry FromAnyGrave")
			}
			found = true
		}
	}
	if !found {
		t.Fatal("precondition: Compost has no Dies trigger")
	}
	if !s.subs[kop.TrigDies].any() {
		t.Fatal("precondition: nothing listens for Dies")
	}
	var m slotMask
	m = m.with(sc.slot(leaving)).with(sc.slot(rats))
	s.npend = 0
	s.leaveAll(k, m, kop.ZoneGraveyard)
	if !sc.onBF(staying) {
		t.Fatal("precondition: the staying Compost left the battlefield")
	}
	if n := sweepPendingBy(sc, staying); n != 1 {
		t.Fatalf("the staying Compost queued %d triggers for the Rat's death, want 1: the filter does not match, so the gate is untested", n)
	}
	if n := sweepPendingBy(sc, leaving); n != 0 {
		t.Fatalf("the departed Compost queued %d from-anywhere triggers on a later member's move, want 0", n)
	}
}
