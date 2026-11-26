package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The granted ripple (CR 702.60a; gorge effects/ripple.go effRipple, the
// trigger gorge's checkGrantedRippleTriggers synthesizes for each derived
// Ripple line past the printed ones): one trigger per granting static
// instance is queued beside the spell as it is cast (queueRipple, called last
// from castFromStack, gorge's granted walk order Demonstrate, Storm,
// Ripple), and its body runs the reveal + may-cast + bottom sequence.
//
// abRipple is 255: every native code below it (247..254) is taken, and the
// value every spell entry uses for "no printed ability" is unreachable on a
// TRIGGER entry -- every reader is either the native class (e.ab >=
// abCascade, 255 included) or a kind-checked spell path. The count N rides
// trigAmt (the first matching static's Static.B, read at queue time) and the
// body reads it as it resolves, like gorge's derived line.
const abRipple uint8 = 255

// rippleProg is the resolution of a ripple trigger (gorge effRipple): reveal
// min(N, library) top cards of the controller's library with one public
// reveal (OpPeek PeekPublic|PeekRemember, the cards stay in the library and
// join the remembered set), offer the revealed cards that share the ripple
// spell's printed name (CR 201.2, read at resolution) as a free cast from
// the library ( gorge's effPlay Optional$ True Amount$ All WithoutManaCost$
// True, casts during resolution, CR 608.2g), then bottom the revealed cards
// still in the library (ExtRippleBottom, the owner ordering two or more).
// The may itself is the CR 603.5 ask the resolution prologue poses
// (resolve.go's abRipple arm): a decline resolves nothing.
var rippleProg = [3]kop.Instr{
	{Op: kop.OpPeek, Obj: kop.RefController, A: kop.ValueRef{K: kop.VTriggerAmount}, C: int16(kop.PeekPublic | kop.PeekRemember)},
	{Op: kop.OpCastFree, Obj: kop.RefRemembered, C: int16(kop.ZoneLibrary) | freeSameName},
	{Op: kop.OpExt, A: kop.Const(int(kop.ExtRippleBottom))},
}

// queueRipple queues one ripple trigger per live StatStackKw Ripple grant of
// the spell e just cast (gorge checkGrantedRippleTriggers: each instance
// separately, CR 702.60b), read with the in-flight cast hidden like Storm
// and Cascade (gorge hasKeywordPriorCasts). The count N is the first
// matching static's B; every instance's trigger carries it, so two grants of
// the same card (two Thrumming Stones) match gorge's two derived lines. A
// spell cast by the ripple itself has the grant too, so the chain ripples
// again: the freeQ drain's cast reaches castFromStack like any cast.
func (s *State) queueRipple(k *prep, e *stackEntry) {
	saved := s.hideInflightCast(e)
	n, b, _ := s.stackKwScan(k, kop.StackKwRipple, e.card, e.obj, e.ctrl, s.castFromHandBy(e, e.ctrl))
	s.restoreInflightCast(e.ctrl, saved)
	if n == 0 {
		return
	}
	for i := 0; i < n; i++ {
		s.enqueue(e.obj, s.objs[e.obj].ver, e.card, abRipple, e.ctrl, e.obj, e.ctrl, b)
	}
}

// rippleBottomExt is OpExt ExtRippleBottom: the ripple body's tail (gorge
// effRippleBottom). Every remembered card still in its owner's library goes
// to the bottom of that library, in remembered order, after the free casts
// took theirs (the tail resumes only once the freeQ drained, so a queued
// cast is either begun -- its card is on the stack -- or refused, its card
// still here). Two or more pose the owner the CR 401.4 arrangement ask over
// the cards still at the top (gorge's libraryPlacementOrder asks BEFORE
// moving: the lockstep compares library positions at the ask), and the
// answer's order is the bottom order (gorge libraryOrderPlacement
// bottom=true, the first pick nearest the top of the group); one or none
// bottoms unasked.
//
//kmanifest:ext ExtRippleBottom
func (s *State) rippleBottomExt(k *prep, in *kop.Instr) (ok, done bool) {
	r := &s.res
	n := 0
	for i := 0; i < int(s.g.nrem); i++ {
		if o := s.g.rem[i]; s.objs[o].zone == kop.ZoneLibrary {
			n++
		}
	}
	if n == 0 {
		return true, true
	}
	if n == 1 {
		s.rippleBottomOne(s.rippleFirstLibCard())
		return true, true
	}
	// gorge's KArrange: Min == Max == the cards offered, no Done, the
	// options in revealed order (the remembered set's order, which is the
	// library's reveal order).
	r.peekN, r.peekI = uint8(n), 0
	r.ask, r.askP = raGenRippleOrder, r.e.ctrl
	return false, true
}

// rippleFirstLibCard is the first remembered card still in the resolving
// controller's library (the one-card tail's move; the multi-card ask's
// options walk the remembered set).
func (s *State) rippleFirstLibCard() Obj {
	for i := 0; i < int(s.g.nrem); i++ {
		if o := s.g.rem[i]; s.objs[o].zone == kop.ZoneLibrary {
			return o
		}
	}
	return NoObj
}

// rippleBottomOne bottoms one card of the ripple reveal (gorge
// libraryOrderPlacement bottom=true for the group it was handed).
func (s *State) rippleBottomOne(o Obj) {
	if o == NoObj {
		return
	}
	from := s.detach(o)
	s.placeFrom(o, kop.ZoneLibrary, true, from, false, false)
}

// rippleOrderOptions lists the group's cards not yet ordered, in the order
// gorge offers them (the remembered set's order, filtered to the library).
func (s *State) rippleOrderOptions(d *Decision) {
	d.Sub = ChArrange
	r := &s.res
	for i := 0; i < int(s.g.nrem); i++ {
		o := s.g.rem[i]
		if s.objs[o].zone != kop.ZoneLibrary {
			continue
		}
		picked := false
		for _, p := range r.arr[:r.peekI] {
			picked = picked || p == o
		}
		if !picked {
			d.add(MakeAction(AkChoose, TCode(o), 0, 0))
		}
	}
}

// rippleOrderAnswer records a pick; the last one bottoms the group in the
// picked order (the first pick nearest the top of the group, gorge
// libraryOrderPlacement bottom=true), each card re-checked in the library
// first (gorge's placed).
func (s *State) rippleOrderAnswer(k *prep, a Action) {
	r := &s.res
	r.arr[r.peekI] = Obj(a.Source())
	r.peekI++
	if r.peekI < r.peekN {
		return // the ask stays posed for the next pick
	}
	for i := 0; i < int(r.peekN); i++ {
		if o := r.arr[i]; s.objs[o].zone == kop.ZoneLibrary {
			s.rippleBottomOne(o)
		}
	}
	r.peekN, r.peekI = 0, 0
	s.continueRes(k, 0)
}
