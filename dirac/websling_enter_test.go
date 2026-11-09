package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestWebSlingedEnterTrigger: an enters trigger's ValidCard$
// Card.Self+wasCast+webSlinged (PredWasCastEnter bit 16) holds for the
// permanent entering from a spell cast for its web-slinging cost, and not
// for a plain cast, a copy of a web-slinging spell (CR 707.10: the copy is a
// token and its entry's mark is cleared, mintSpellCopy) or another object.
func TestWebSlingedEnterTrigger(t *testing.T) {
	var s State
	const o, other = Obj(3), Obj(4)
	webSlinged := &kop.Filter{Pred: kop.PredWasCastEnter, PredN: wasCastWebSling}
	plain := &kop.Filter{Pred: kop.PredWasCastEnter, PredN: 0}
	spell := func(trigPl uint8) {
		s.res.active = true
		s.res.e = stackEntry{kind: seSpell, obj: o, ctrl: 0, prov: uint8(kop.ZoneHand), trigPl: trigPl}
	}

	spell(trigPlWebSling)
	if !webSlingEntry(&s.res.e) || !s.wasCastPred(plain, o, 0) {
		t.Fatal("precondition: the web-slinging entry is not a cast spell entry")
	}
	if !s.wasCastPred(webSlinged, o, 0) {
		t.Error("a web-slinging cast: the trigger does not fire")
	}
	if s.wasCastPred(webSlinged, other, 0) {
		t.Error("another object entering read as web-slinged")
	}

	spell(0)
	if !s.wasCastPred(plain, o, 0) {
		t.Fatal("precondition: a plain cast does not read as cast")
	}
	if s.wasCastPred(webSlinged, o, 0) {
		t.Error("a plain cast fired the web-slinging trigger")
	}

	// a stack copy of the web-slinging spell: mintSpellCopy clears the mark
	// and the object is a copy (a token by the time it enters)
	spell(0)
	s.objs[o].of = ofCopy
	if s.wasCastPred(webSlinged, o, 0) {
		t.Error("a copy fired the web-slinging trigger")
	}
	spell(trigPlWebSling)
	s.objs[o].of = ofToken
	if s.wasCastPred(webSlinged, o, 0) {
		t.Error("a copy that kept the mark read as cast")
	}
	s.objs[o].of = 0
	s.res.active = false
	if s.wasCastPred(webSlinged, o, 0) {
		t.Error("no resolution in flight read as web-slinged")
	}
}
