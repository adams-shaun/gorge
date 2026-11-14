package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// Two Equipment that each grant an attack trigger queue them in layer
// timestamp order, and an Equipment takes a new timestamp when it is attached
// (CR 613.7e): the one attached first queues first whichever entered first, so
// the controller's trigger-order answer means the same pair of triggers on
// both sides.
func TestGrantedAttackTriggersQueueInAttachOrder(t *testing.T) {
	sc, id := graveActScen(t, "Hunter's Axe", "Medic's Kitesail", "Woodwork Prodigy", "Forest")
	axe := sc.perm(id["Hunter's Axe"], 0)
	sail := sc.perm(id["Medic's Kitesail"], 0)
	bear := sc.perm(id["Woodwork Prodigy"], 0)
	sc.s.attachTo(sc.k, sc.slot(sail), sc.slot(bear))
	sc.s.attachTo(sc.k, sc.slot(axe), sc.slot(bear))
	sc.passUntil(t, kop.StepAttackers)
	sc.do(t, AkAttack, TCode(bear), scAny)
	sc.do(t, AkDone, scAny, scAny)
	d := sc.s.Step()
	if d.Kind != DkTriggerOrder || sc.s.npend != 2 {
		t.Fatalf("decision kind %d with %d pending, want a trigger order of 2", d.Kind, sc.s.npend)
	}
	if got, want := [2]CardID{sc.s.pend[0].card, sc.s.pend[1].card}, [2]CardID{id["Medic's Kitesail"], id["Hunter's Axe"]}; got != want {
		t.Fatalf("queued cards %v, want %v: the Kitesail was attached first", got, want)
	}
}
