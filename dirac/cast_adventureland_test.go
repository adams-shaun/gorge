package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCounteredAdventureNotOnAdventure: an Adventure spell countered to
// exile is not on an adventure (CR 715.3d, gorge): it leaves the
// stack as its front, a nonland front gets no recast, a land front gets no
// land play, even with the turn's land drop unused. The faces pool has no
// counter-to-exile card, so the test drives counterTo (Dissipate's
// Destination$ exile path). Kernel models no graveyard-to-exile
// replacement (repl_gen.go graveRedirect only redirects to the library).
func TestCounteredAdventureNotOnAdventure(t *testing.T) {
	t.Run("nonland front", func(t *testing.T) {
		sc, id := facesScen(t, "Murderous Rider", "Grizzly Bears", "Swamp")
		front := id["Murderous Rider"]
		for i := 0; i < 6; i++ {
			sc.perm(id["Swamp"], 0)
		}
		sc.perm(id["Grizzly Bears"], 1)
		mr := sc.hand(front, 0)
		sc.s.Step()
		sc.s.Apply(MakeAction(AkCast, TCode(mr), 1, castFace))
		sc.buildToStack(t)
		if i := sc.s.stackIndexOf(mr); i >= 0 {
			sc.s.counterTo(sc.k, i, kop.CtrDestExile)
		} else {
			t.Fatal("object not on the stack")
		}
		ob := &sc.s.objs[mr]
		if ob.zone != kop.ZoneExile || ob.card != front || ob.of&ofAdventure != 0 {
			t.Fatalf("countered to exile: zone %d, front %v, on an adventure %v",
				ob.zone, ob.card == front, ob.of&ofAdventure != 0)
		}
		d := sc.s.Step()
		if got := faceCasts(&d, mr); len(got) != 0 {
			t.Fatalf("recasts %v, want none after a counter", got)
		}
	})

	t.Run("land front", func(t *testing.T) {
		sc, id := facesScen(t, "Lindblum, Industrial Regency", "Mountain")
		front := id["Lindblum, Industrial Regency"]
		for i := 0; i < 3; i++ {
			sc.perm(id["Mountain"], 0)
		}
		lb := sc.hand(front, 0)
		sc.s.Step()
		sc.s.Apply(MakeAction(AkCast, TCode(lb), 1, castFace))
		sc.buildToStack(t)
		if i := sc.s.stackIndexOf(lb); i >= 0 {
			sc.s.counterTo(sc.k, i, kop.CtrDestExile)
		} else {
			t.Fatal("object not on the stack")
		}
		ob := &sc.s.objs[lb]
		if ob.zone != kop.ZoneExile || ob.card != front || ob.of&ofAdventure != 0 {
			t.Fatalf("countered to exile: zone %d, front %v, on an adventure %v",
				ob.zone, ob.card == front, ob.of&ofAdventure != 0)
		}
		sc.s.pl[0].landsPlayed = 0
		d := sc.s.Step()
		if got := landPlays(&d, lb); len(got) != 0 {
			t.Fatalf("land plays %v, want none after a counter", got)
		}
		if got := faceCasts(&d, lb); len(got) != 0 {
			t.Fatalf("recasts %v, want none for a land", got)
		}
	})
}
