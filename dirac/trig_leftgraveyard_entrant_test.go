package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestGraveWatcherEnteredInBatchLooksBack: a permanent that enters the
// battlefield in a graveyard sweep does not watch a sibling leave the same
// sweep. Fang, Fearless l'Cie is milled and returned by Random Encounter
// beside the card moving after it; looked at from the board before the sweep
// it was a card in the graveyard, so it sees nothing (CR 603.10a). A watcher
// already on the battlefield, Chalk Outline, still sees the batch once.
func TestGraveWatcherEnteredInBatchLooksBack(t *testing.T) {
	sc, id := newLeftGraveyardScen(t, []cardset.Entry{
		{Name: "Chalk Outline", Count: 1}, {Name: "Fang, Fearless l'Cie", Count: 1},
		{Name: "Grizzly Bears", Count: 1}, {Name: "Forest", Count: 2}, {Name: "Plains", Count: 8},
	})
	chalkID, fangID, bearsID := id["Chalk Outline"], id["Fang, Fearless l'Cie"], id["Grizzly Bears"]
	chalk := sc.perm(chalkID, 0)
	chalkAb := trigLeftgraveyardGraveAb(t, sc, chalkID)
	fangAb := trigLeftgraveyardGraveAb(t, sc, fangID)
	fang := sc.grave(fangID, 0, 1)[0]
	bears := sc.grave(bearsID, 0, 1)[0]
	if sc.s.objs[bears].zone != kop.ZoneGraveyard {
		t.Fatal("precondition: the sibling is not in the graveyard")
	}
	sc.s.npend = 0
	stamp := sc.s.ts
	old := sc.s.batchOpen()
	if sc.s.g.batchLatch&batchOpenMask == 0 {
		t.Fatal("precondition: the sweep batch did not open")
	}
	from := sc.s.detach(fang)
	sl := sc.s.enter(sc.k, fang, 0, false, from)
	if sc.s.perms[sl].obj != fang {
		t.Fatal("precondition: Fang did not enter")
	}
	if sc.s.perms[sl].ts <= stamp {
		t.Fatal("precondition: Fang entered before the batch opened")
	}
	sc.s.detach(bears)
	sc.s.batchClose(old)
	if n := batchPend(sc, chalk, chalkAb); n != 1 {
		t.Fatalf("the watcher already on the battlefield queued %d triggers, want 1", n)
	}
	if n := batchPend(sc, fang, fangAb); n != 0 {
		t.Fatalf("the entrant queued %d triggers for the sibling's leave, want 0", n)
	}
}
