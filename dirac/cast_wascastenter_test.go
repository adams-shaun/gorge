package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWasCastEnterReads: PredWasCastEnter holds for the object the resolving
// spell entry is becoming and no other (a flicker, a reanimation or a token
// arrives as an ability's object), reads the cast zone from the entry, and
// never holds for a copy (CR 707.10). The trigger carriers (no real card
// lowers them yet) share this one read with the enters replacement.
func TestWasCastEnterReads(t *testing.T) {
	var s State
	const o, other = Obj(3), Obj(4)
	pred := func(n int16) *kop.Filter { return &kop.Filter{Pred: kop.PredWasCastEnter, PredN: n} }
	spell := func(prov, flags uint8) {
		s.res.active = true
		s.res.e = stackEntry{kind: seSpell, obj: o, ctrl: 0, prov: prov, flags: flags}
	}
	spell(uint8(kop.ZoneHand), 0)
	for _, c := range []struct {
		name string
		fl   *kop.Filter
		o    Obj
		ctrl uint8
		want bool
	}{
		{"wasCast, the resolving spell", pred(0), o, 0, true},
		{"!wasCast, the resolving spell", pred(4), o, 0, false},
		{"wasCast, another object", pred(0), other, 0, false},
		{"!wasCast, another object", pred(4), other, 0, true},
		{"from graveyard, a hand cast", pred(1), o, 0, false},
		{"from hand by you", pred(2), o, 0, true},
		{"from hand, controlled by the other player", pred(2), o, 1, false},
		{"!from hand by you", pred(2 | 4), o, 0, false},
	} {
		if got := s.wasCastPred(c.fl, c.o, c.ctrl); got != c.want {
			t.Errorf("%s: %v, want %v", c.name, got, c.want)
		}
	}
	spell(uint8(kop.ZoneGraveyard), 0)
	if !s.wasCastPred(pred(1), o, 0) || s.wasCastPred(pred(2), o, 0) {
		t.Error("a graveyard cast: wasCastFromGraveyard false or from-hand true")
	}
	spell(0, sfFlashback)
	if !s.wasCastPred(pred(1), o, 0) {
		t.Error("a flashback cast read as not from a graveyard")
	}
	s.objs[o].of = ofCopy
	if s.wasCastPred(pred(0), o, 0) || !s.wasCastPred(pred(4), o, 0) {
		t.Error("a copy read as cast")
	}
	s.objs[o].of = ofToken // a permanent spell's copy, flipped to a token as it enters
	if s.wasCastPred(pred(0), o, 0) || !s.wasCastPred(pred(4), o, 0) {
		t.Error("a copy that has become a token read as cast")
	}
	s.objs[o].of = 0
	s.res.e.kind = seTrigger
	if s.wasCastPred(pred(0), o, 0) {
		t.Error("an ability's entry read as a cast")
	}
	s.res.active = false
	spell(uint8(kop.ZoneHand), 0)
	s.res.active = false
	if s.wasCastPred(pred(0), o, 0) {
		t.Error("no resolution in flight read as cast")
	}
}
