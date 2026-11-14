package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCrimePickVerdictCarriesToPlacement is CR 700.13's verdict site: the
// crime is judged when the target was targeted (CR 601.2c, before the
// 601.2h cost), so checkWard's walk (crimeStep) reads the verdict the ask
// recorded (crimePick/crimeAskDone) instead of re-judging a target a cost
// or a trigger moved between the pick and the placement -- Boom Box's own
// activation targets itself and sacrifices it, so at placement the target
// sits in its owner's graveyard, a crime only if judged there. Both
// directions, plus the fallback for an entry nothing recorded verdicts on.
func TestCrimePickVerdictCarriesToPlacement(t *testing.T) {
	sc, id := crimeScen(t)
	s := sc.s
	// the stolen target is a land: no abilities of its own, so the only
	// queued triggers are the two crime listeners'.
	_, _ = sc.perm(id("Marchesa, Dealer of Death"), 0), sc.perm(id("Rattleback Apothecary"), 0)
	// Boom Box's shape: their card (owner 1) under my control, the target
	// of my own ability's pick.
	stolen := sc.perm(id("Swamp"), 1)
	s.perms[sc.slot(stolen)].ctrl = 0
	if s.objs[stolen].owner() != 1 {
		t.Fatal("the land is not their card")
	}
	// run picks (or not), then a cost moves the target out of my
	// battlefield, then the placement walk.
	run := func(pick uint8, picked bool, moveTo kop.Zone) int {
		s.npend, s.g.crime = 0, 0
		*s.bld() = build{e: stackEntry{kind: seAbility, ctrl: 0, obj: stolen, trigObj: NoObj}}
		s.bld().e.tgt = noTargets
		s.bld().e.tgt[0] = TCode(pick)
		s.bld().e.nt = 1
		if picked {
			// the pick, through the ask machinery: CR 700.13 judges it
			// here (a slot 0 pick, crimePick's done), and the verdict
			// rides the entry into the walk
			s.crimePick(sc.k, TCode(pick), true)
		}
		if moveTo != kop.ZoneNone {
			s.place(stolen, moveTo, false)
		}
		e := s.bld().e
		s.checkWard(sc.k, &e)
		return int(s.npend)
	}
	// the pick was no crime (a battlefield permanent I control); by
	// placement the sacrificed target sits in my graveyard, owned by the
	// opponent -- a crime only if re-judged there.
	if s.perms[sc.slot(stolen)].ctrl != 0 {
		t.Fatal("the land is not mine at the pick")
	}
	if n := run(uint8(stolen), true, kop.ZoneGraveyard); n != 0 {
		t.Fatalf("the pick's non-crime re-judged at placement: %d triggers queued, want 0", n)
	}
	if s.objs[stolen].zone != kop.ZoneGraveyard || s.objs[stolen].owner() != 1 {
		t.Fatalf("the land is %v owned %d, want their graveyard card", s.objs[stolen].zone, s.objs[stolen].owner())
	}
	if n := run(uint8(stolen), false, kop.ZoneNone); n != 2 {
		t.Fatalf("an unrecorded walk: %d triggers, want 2 (one per listener; judged here)", n)
	}
	// the other direction: the pick WAS a crime (their battlefield
	// permanent) and a cost exiled the target, which is never a crime
	// there -- the recorded verdict still fires.
	stolen = sc.perm(id("Swamp"), 1)
	if s.perms[sc.slot(stolen)].ctrl != 1 {
		t.Fatal("the land is not theirs at the pick")
	}
	if n := run(uint8(stolen), true, kop.ZoneExile); n != 2 {
		t.Fatalf("the recorded crime: %d triggers, want 2 (one per listener)", n)
	}
	if s.objs[stolen].zone != kop.ZoneExile {
		t.Fatal("the land is not in exile at placement")
	}
	if n := run(uint8(stolen), false, kop.ZoneNone); n != 0 {
		t.Fatalf("an unrecorded walk: %d triggers, want 0 (exile is never a crime)", n)
	}
}
