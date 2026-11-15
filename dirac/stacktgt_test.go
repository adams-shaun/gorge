package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFlashbackTargetGoneOnStackReverses: Consuming Aberration's power is
// the cards in its controller's opponents' graveyards. With four in p0's,
// it is a legal target of Make Your Move ("creature with power 4 or
// greater"), so the flashback cast is offered; moving Make Your Move to the
// stack (CR 601.2a) leaves three, and at the target ask (CR 601.2c) the
// Aberration is a 3/3. The cast is reversed (CR 733.1, gorge's "cast
// aborted: no legal target"), the card back at the end of the graveyard;
// before, the kernel panicked "spell/ability without a legal target"
// (random-policy FDN game 249014 of a random-deck hunt).
func TestFlashbackTargetGoneOnStackReverses(t *testing.T) {
	sc, id := newObjScen(t, []cardset.Entry{{Name: "Make Your Move", Count: 1}, {Name: "Consuming Aberration", Count: 1}, {Name: "Sphinx of Forgotten Lore", Count: 1}, {Name: "Plains", Count: 8}})
	for i := 0; i < 3; i++ {
		sc.perm(id["Plains"], 0)
	}
	sc.perm(id["Consuming Aberration"], 1)
	mym := sc.obj(id["Make Your Move"], 0)
	sc.s.place(mym, kop.ZoneGraveyard, false)
	sc.grave(id["Plains"], 0, 3)
	sc.s.grantFB(sc.k, mym)
	d := sc.s.Step()
	var cast Action
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkCast && a.Source() == TCode(uint8(mym)) {
			cast = a
		}
	}
	if cast == 0 {
		t.Fatalf("flashback Make Your Move not offered: kind %d %v", d.Kind, d.Options[:d.N])
	}
	sc.s.Apply(cast)
	d = sc.s.Step()
	pl := &sc.s.pl[0]
	if d.Kind != DkPriority || sc.s.objs[mym].zone != kop.ZoneGraveyard || pl.grave[pl.graveN-1] != mym || sc.s.sp != 0 {
		t.Fatalf("cast not reversed: decision kind %d, zone %d, stack %d", d.Kind, sc.s.objs[mym].zone, sc.s.sp)
	}
	for m := sc.s.ctrlM[0]; m.any(); m = m.rest() {
		if sc.s.perms[m.first()].flags&pfTapped != 0 {
			t.Fatal("a land was tapped for the reversed cast")
		}
	}
}
