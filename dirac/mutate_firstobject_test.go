package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// TestMutateTargetIsFirstObject: card instance 0 (the first card of seat 0's
// deck; no object is NoObj) is a legal mutate target like any other (CR
// 702.140a). The mutate cost is hybrid ({U/R}{U/R}, CR 107.4e): the cast is
// offered, announces object 0 and merges onto it.
func TestMutateTargetIsFirstObject(t *testing.T) {
	tbl, mutator, bird, _, _ := mutateTable()
	ur := kop.ColU | kop.ColR
	tbl.Cards[mutator].MutateCost = kop.ManaCost{Hyb: [5]kop.ColorMask{ur, ur}}
	sc := newScenTableLib(t, tbl, bird, Config{NoElide: true})
	const first = Obj(0)
	if ob := &sc.s.objs[first]; ob.card != bird || ob.owner() != 0 || ob.zone != kop.ZoneLibrary {
		t.Fatalf("precondition: object 0 is card %d owner %d zone %d, want seat 0's library bird", ob.card, ob.owner(), ob.zone)
	}
	sc.s.detach(first)
	sc.s.enter(sc.k, first, 0, false, kop.ZoneNone)
	sc.s.npend = 0
	sc.lands(0, v.Island, 1)
	sc.lands(0, v.Mountain, 1)
	mut := sc.hand(mutator, 0)
	if mut == first || !sc.onBF(first) {
		t.Fatalf("precondition: the bird (object %d) is the only mutate target on the battlefield", first)
	}

	d := sc.s.Step()
	if d.Kind != DkPriority {
		t.Fatalf("decision kind %d, want priority", d.Kind)
	}
	if !mutateOffered(d, mut) {
		t.Fatalf("mutate cast onto object 0 not offered: %v", d.Options[:d.N])
	}
	sc.do(t, AkMutate, TCode(mut), scAny)
	mutatePlace(t, sc, true)
	// the hybrid pips are announced before the target (CR 601.2b, 601.2c)
	for i := 0; i < 2; i++ {
		ask := sc.s.Step()
		if ask.N == 0 || ask.Options[0].Kind() == AkTarget {
			break
		}
		sc.s.Apply(ask.Options[0])
	}
	sc.do(t, AkTarget, scAny, TCode(first))
	resolveStack(t, sc)
	if rec := sc.s.mergeRecOf(first); rec == nil || rec.n != 1 || sc.s.cardAt(sc.slot(first)) != mutator {
		t.Fatalf("object 0 is not a pile with the mutator on top")
	}
}
