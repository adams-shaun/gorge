package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestFabledPassageUntaps (kmanaprog; gorge zone_search RememberChanged$
// eventRemember): Fabled Passage's search remembers the land it puts onto
// the battlefield tapped, so "then if you control four or more lands, untap
// that land" untaps it with four lands and leaves it tapped with three.
func TestFabledPassageUntaps(t *testing.T) {
	for _, plains := range []int{2, 3} {
		sc, id := newBLBScen(t, []cardset.Entry{{Name: "Fabled Passage", Count: 1}, {Name: "Plains", Count: 8}})
		fp := sc.perm(id["Fabled Passage"], 0)
		sc.lands(0, id["Plains"], plains)
		before := sc.s.pl[0].libLen()
		if !sc.activateAb(fp, 0) {
			t.Fatal("Fabled Passage's ability not offered")
		}
		sc.passBoth(t)
		d := sc.expect(t, DkChoose, ChGenSearch)
		found, ok := Obj(0), false
		for i := int(d.N) - 1; i >= 0; i-- {
			if o := Obj(d.Options[i].Source()); sc.s.objs[o].zone == kop.ZoneLibrary {
				found, ok = o, true
				sc.s.Apply(d.Options[i])
				break
			}
		}
		if !ok {
			t.Fatalf("no library card offered: %v", d.Options[:d.N])
		}
		sc.s.Step()
		if sc.s.pl[0].libLen() != before-1 || !sc.onBF(found) {
			t.Fatalf("%d plains: the fetched land is not on the battlefield", plains)
		}
		if want := plains < 3; sc.tapped(found) != want {
			t.Fatalf("%d plains + the fetched land: tapped %v, want %v", plains, sc.tapped(found), want)
		}
	}
}
