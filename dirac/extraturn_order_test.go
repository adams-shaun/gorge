package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestExtraTurnRiderQueuesBeforeLaterPromises: gorge registers the granted
// turn's loss at the consumption, before a "next end step" promise made
// during the granted turn, so it queues ahead of it. The drain pushes the
// first queued first, so the promise resolves before the loss: the rider
// must sit at a lower pend index than the delayed trigger.
func TestExtraTurnRiderQueuesBeforeLaterPromises(t *testing.T) {
	sc, id := riderScenario(t, []cardset.Entry{{Name: "Final Fortune", Count: 1}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4}})
	s := sc.s
	ff := sc.hand(id["Final Fortune"], 0)
	tok := sc.perm(id["Mountain"], 0)
	s.exRideArm(0, ff)
	s.dly[0] = delayed{act: uint8(kop.OpSacrifice), ctrl: 0, obj: tok, ver: objWord(s.objs[tok].ver), creator: NoObj}
	s.ndly = 1
	s.npend = 0
	s.firePhase(sc.k, kop.StepEnd)
	s.fireDelayedAt(sc.k, 0)
	if s.npend != 2 {
		t.Fatalf("%d queued, want the rider and the promise", s.npend)
	}
	if s.pend[0].trigAmt != exTurnRiderAmt || s.pend[1].obj != tok {
		t.Fatalf("queue [amt %d obj %d, amt %d obj %d]: the rider must precede the later promise", s.pend[0].trigAmt, s.pend[0].obj, s.pend[1].trigAmt, s.pend[1].obj)
	}
}

// TestExtraTurnRiderKeepsArmedIncarnation: the rider's stack entry names the
// source as it stood when the rider armed, not a recycled slot's later one.
func TestExtraTurnRiderKeepsArmedIncarnation(t *testing.T) {
	sc, id := riderScenario(t, []cardset.Entry{{Name: "Final Fortune", Count: 1}, {Name: "Mountain", Count: 4}, {Name: "Island", Count: 4}})
	s := sc.s
	ff := sc.hand(id["Final Fortune"], 0)
	ver, card := uint8(s.objs[ff].ver), s.objs[ff].card
	s.exRideArm(0, ff)
	s.objs[ff].ver++
	s.objs[ff].card = id["Island"]
	s.npend = 0
	s.exRideFire()
	if s.npend != 1 || s.pend[0].srcVer != ver || s.pend[0].card != card {
		t.Fatalf("pending %d ver %d card %d, want ver %d card %d", s.npend, s.pend[0].srcVer, s.pend[0].card, ver, card)
	}
}
