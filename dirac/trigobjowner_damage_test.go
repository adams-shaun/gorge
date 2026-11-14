package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestTriggerObjOwnerIsADamagePlayer: kop.RefTriggerObjOwner (gorge Defined$
// TriggeredCardOwner) is a player ref, so a DealDamage carrying it damages
// the trigger object's owner (gorge definedSpecTriggeredCardOwner hands
// DealDamage that player), never the object itself or nobody.
func TestTriggerObjOwnerIsADamagePlayer(t *testing.T) {
	sc := newScen(t, Config{NoElide: true})
	s := sc.s

	// the trigger object: owned by seat 1, controlled by seat 0
	obj := sc.perm(v.GrizzlyBears, 0)
	s.objs[obj].setOwner(1)
	s.res.e = stackEntry{kind: seTrigger, trigObj: obj, trigPl: 0, ctrl: 0}

	// precondition: the ref names the owner, which is not the controller
	if got := s.playerRef(sc.k, kop.RefTriggerObjOwner); got != 1 {
		t.Fatalf("playerRef: %d, want the trigger object's owner 1", got)
	}
	if got, want := s.refCode(sc.k, kop.RefTriggerObjOwner), PlayerCode(1); got != want {
		t.Fatalf("refCode: %d, want player 1's code %d (object %d)", got, want, obj)
	}

	// no trigger object bound: no damage target
	s.res.e.trigObj = NoObj
	if got := s.refCode(sc.k, kop.RefTriggerObjOwner); got != NoTarget {
		t.Fatalf("refCode with no trigger object: %d, want NoTarget", got)
	}
}
