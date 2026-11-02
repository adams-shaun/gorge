package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// newKturnuprepScen is newScenTable over the turnuprepl pool: a library of
// Zoetic Caverns, the four TurnFaceUp carriers by name.
func newKturnuprepScen(t *testing.T) (*scen, map[string]CardID) {
	t.Helper()
	deck := []cardset.Entry{
		{Name: "Karlov Watchdog", Count: 1},
		{Name: "Bubble Smuggler", Count: 3},
		{Name: "Crowd-Control Warden", Count: 1},
		{Name: "Unable to Scream", Count: 1},
		{Name: "Zoetic Cavern", Count: 20},
	}
	set := cardsettest.Set(t, "turnuprepl")
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	lib := make([]CardID, 20)
	for i := range lib {
		lib[i] = id["Zoetic Cavern"]
	}
	s := new(State)
	s.Init(tbl, lib, lib, 1, Config{NoElide: true})
	for p := uint8(0); p < 2; p++ {
		pl := &s.pl[p]
		for pl.handN > 0 {
			o := pl.hand[0]
			s.detach(o)
			s.objs[o].zone = kop.ZoneLibrary
			pl.libBottom(o)
		}
	}
	s.turn, s.active, s.step, s.prio, s.passes, s.ctl = 2, 0, kop.StepMain1, 0, 0, ctlPriority
	return &scen{s: s, k: s.k()}, id
}

func (sc *scen) turnUpOffers(p uint8) int {
	var d Decision
	sc.s.offerTurnFaceUp(sc.k, &d, p)
	return int(d.N)
}

// Karlov Watchdog (ReplCantTurnUp, Filter an opponent's permanent, Cond
// your turn): the opponent's face-down permanent is not offered a turn-up
// on the Watchdog controller's turn, is offered on its own, and the
// Watchdog's controller is never held. A turn-up submitted anyway pays
// nothing and does nothing.
func TestTurnuprepWatchdogCantHappen(t *testing.T) {
	sc, id := newKturnuprepScen(t)
	sc.perm(id["Karlov Watchdog"], 0)
	mine := sc.obj(id["Zoetic Cavern"], 0)
	sc.s.enterFaceDown(sc.k, mine, 0, kop.MorphMorph)
	theirs := sc.obj(id["Zoetic Cavern"], 1)
	sl := sc.s.enterFaceDown(sc.k, theirs, 1, kop.MorphMorph)
	sc.s.pl[0].pool[kop.C] = 2
	sc.s.pl[1].pool[kop.C] = 2

	sc.s.active = 0
	if n := sc.turnUpOffers(1); n != 0 {
		t.Errorf("opponent offered %d turn-ups on the Watchdog controller's turn", n)
	}
	if n := sc.turnUpOffers(0); n != 1 {
		t.Errorf("Watchdog controller offered %d turn-ups, want 1", n)
	}
	sc.s.turnFaceUp(sc.k, theirs, 1)
	if !sc.s.faceDown(sc.k, sl) || sc.s.pl[1].pool[kop.C] != 2 {
		t.Error("a submitted turn-up on the Watchdog controller's turn paid or turned up")
	}
	sc.s.active = 1
	if n := sc.turnUpOffers(1); n != 1 {
		t.Errorf("opponent offered %d turn-ups on its own turn, want 1", n)
	}
	sc.s.turnFaceUp(sc.k, theirs, 1)
	if sc.s.faceDown(sc.k, sl) || sc.s.pl[1].pool[kop.C] != 0 {
		t.Error("the turn-up on the opponent's own turn did not happen")
	}
}

// Bubble Smuggler's body: four +1/+1 counters land on it as it turns up.
// Crowd-Control Warden's: one per other creature its controller controls.
func TestTurnuprepCounters(t *testing.T) {
	sc, id := newKturnuprepScen(t)
	o := sc.obj(id["Bubble Smuggler"], 0)
	sl := sc.s.enterFaceDown(sc.k, o, 0, kop.MorphDisguise)
	sc.s.pl[0].pool[kop.U], sc.s.pl[0].pool[kop.C] = 1, 5
	sc.s.turnFaceUp(sc.k, o, 0)
	if sc.s.faceDown(sc.k, sl) || sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)) != 4 {
		t.Errorf("Bubble Smuggler: face down %v, counters %d, want face up with 4", sc.s.faceDown(sc.k, sl), sc.s.perms[sl].ctrLow(int(kop.CtrP1P1)))
	}

	// a manifested Warden (printed mana cost {3}{G}{W}) beside two other
	// creatures, one of them face down
	sc.perm(id["Bubble Smuggler"], 0)
	other := sc.obj(id["Bubble Smuggler"], 0)
	sc.s.enterFaceDown(sc.k, other, 0, kop.MorphDisguise)
	w := sc.obj(id["Crowd-Control Warden"], 0)
	wsl := sc.s.enterFaceDown(sc.k, w, 0, kop.MorphDisguise)
	sc.s.markFaceDown(wsl, false)
	sc.s.pl[0].pool = [kop.NumManaKinds]uint8{}
	sc.s.pl[0].pool[kop.G], sc.s.pl[0].pool[kop.W], sc.s.pl[0].pool[kop.C] = 1, 1, 3
	sc.s.turnFaceUp(sc.k, w, 0, 1)
	// the first Smuggler (4 counters, face up), the second (face up) and the
	// face-down one are the Warden's three other creatures
	if got := sc.s.perms[wsl].ctrLow(int(kop.CtrP1P1)); sc.s.faceDown(sc.k, wsl) || got != 3 {
		t.Errorf("Crowd-Control Warden: face down %v, counters %d, want face up with 3", sc.s.faceDown(sc.k, wsl), got)
	}
}
