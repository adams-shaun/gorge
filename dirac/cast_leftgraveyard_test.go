package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// kactytRing counts the ring's {o, stack, graveyard} entries: a cast from a
// graveyard.
func kactytRing(s *State, o Obj) int {
	n := 0
	for i := uint8(0); i < s.g.enteredN; i++ {
		if e := &s.cold.entered[i]; e.o == o && e.to == uint8(kop.ZoneStack) && e.from == uint8(kop.ZoneGraveyard) {
			n++
		}
	}
	return n
}

// kactytReversedCast drives a flashback Eaten Alive cast to its unpayable
// either-or part (the CR 733.1 reversal of TestEitherOrUndoFromGraveyard) and
// returns the state with the card back in the graveyard. ringAtPush is the
// number of graveyard-cast entries while the spell was on the stack and
// ringBefore the ring's length before the cast.
func kactytReversedCast(t *testing.T) (s *State, ea Obj, ringAtPush int, ringBefore uint8) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Eaten Alive", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Burglar Rat", Count: 2}, {Name: "Swamp", Count: 8}})
	sc.s.cfg |= cfgGorgeAltAdd
	sc.s.enteredRing() // the ring a LeftGraveyardThisTurn count reads
	sc.perm(id["Swamp"], 0)
	sc.perm(id["Burglar Rat"], 0)
	sc.perm(id["Burglar Rat"], 1)
	ea = sc.obj(id["Eaten Alive"], 0)
	sc.s.place(ea, kop.ZoneGraveyard, false)
	sc.s.grantFB(sc.k, ea)
	ringBefore = sc.s.g.enteredN
	sc.stepTo(t, 1, func() bool { return false }, func(d *Decision) (Action, bool) {
		for i := 0; i < int(d.N); i++ {
			if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(ea)) {
				return a, true
			}
		}
		t.Fatalf("flashback Eaten Alive not offered: %v", d.Options[:d.N])
		return 0, false
	})
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChAltAdd {
		t.Fatalf("altadd decision kind %d sub %d", d.Kind, d.Sub)
	}
	ringAtPush = kactytRing(sc.s, ea)
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Target() == 1 {
			sc.s.Apply(d.Options[i])
		}
	}
	if d := sc.s.Step(); d.Kind != DkPriority || sc.s.objs[ea].zone != kop.ZoneGraveyard {
		t.Fatalf("the unpayable part did not reverse the cast: kind %d zone %d", d.Kind, sc.s.objs[ea].zone)
	}
	return sc.s, ea, ringAtPush, ringBefore
}

// TestActytReversedCastDropsDeparture: a flashback cast is a graveyard
// departure while it sits on the stack, and its reversal (CR 733.1) takes the
// departure back: gorge truncates g.Entered to the push-time length
// (events/apply_zone.go), so Count$LeftGraveyardThisTurn does not count it.
func TestActytReversedCastDropsDeparture(t *testing.T) {
	s, ea, atPush, before := kactytReversedCast(t)
	if atPush != 1 {
		t.Fatalf("%d graveyard-cast entries while the spell is on the stack, want 1", atPush)
	}
	if n := kactytRing(s, ea); n != 0 || s.g.enteredN != before {
		t.Fatalf("after the reversal the ring holds %d cast entries (%d total, %d before the cast), want none", n, s.g.enteredN, before)
	}
}

// TestActytReversedCastMutation: the check above sees a ring that kept the
// entry (the pre-fix behaviour: noteCastFromGrave without its undo).
func TestActytReversedCastMutation(t *testing.T) {
	s, ea, _, _ := kactytReversedCast(t)
	s.noteCastFromGrave(ea)
	if kactytRing(s, ea) == 0 {
		t.Fatal("mutation not detected: a stale cast entry was not seen in the ring")
	}
}
