package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMorphTurnUpXAnnounced: Aurelia's Vindicator (Disguise {X}{3}{W}) face
// down with {W}{C}{C}{C}{C} floating is offered the turn-up; the action asks
// X first (0 and 1 only: X = 2 needs {5}{W}), pays it, and the TurnFaceUp
// trigger is queued with the announced X bound (CR 107.3m).
func TestMorphTurnUpXAnnounced(t *testing.T) {
	deck := []cardset.Entry{
		{Name: "Aurelia's Vindicator", Count: 4},
		{Name: "Mox Jet", Count: 36},
	}
	set := cardsettest.Set(t, "facedown")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mox Jet"], Config{NoElide: true})
	o := sc.obj(id["Aurelia's Vindicator"], 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphDisguise)
	sc.s.npend = 0
	sc.s.pl[0].pool[kop.W], sc.s.pl[0].pool[kop.C] = 1, 4
	if !sc.s.faceDown(sc.k, sl) {
		t.Fatal("precondition: Aurelia is face down")
	}
	if n := sc.turnUpOffers(0); n != 1 {
		t.Fatalf("turn-up offers = %d, want 1", n)
	}
	sc.s.Apply(MakeAction(AkTurnFaceUp, TCode(o), 0, 0))
	if !sc.s.faceDown(sc.k, sl) {
		t.Fatal("the turn-up completed before X was announced")
	}
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChX || d.N != 2 || d.Options[0].Mode() != 0 || d.Options[1].Mode() != 1 {
		t.Fatalf("X ask = kind %d sub %d options %v, want ChX {0,1}", d.Kind, d.Sub, d.Options[:d.N])
	}
	sc.s.Apply(d.Options[1])
	if sc.s.faceDown(sc.k, sl) {
		t.Fatal("Aurelia is still face down after X = 1")
	}
	if p := sc.s.pl[0].pool; p[kop.W] != 0 || p[kop.C] != 0 {
		t.Fatalf("pool after paying X = 1: %v, want empty", p)
	}
	if sc.s.npend != 1 || sc.s.pend[0].x != 1 || sc.s.pend[0].trigAmt != 1 {
		t.Fatalf("pending triggers: n=%d, want one with x = 1, trigAmt = 1 (%+v)", sc.s.npend, sc.s.pend[0])
	}
}
