package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// An emblem that remembers a target (kop.Emblem.Capture; State.g.embN holds
// the object + 1) applies its statics to that object alone, and, with
// kop.Emblem.CaptureLapse, stops applying them once the object leaves the
// battlefield (the Effect's ForgetOnMoved$ / ExileOnMoved$ Battlefield).

// embCaptureOK reports whether the emblem statSrc.emb names (a handle:
// player<<4 | index+1) may apply to the permanent at sl: an emblem with no
// capture applies to anything its filter matches; one with a capture, only to
// the captured object.
func (s *State) embCaptureOK(k *prep, emb uint8, sl Slot) bool {
	p, i := emb>>4, int(emb&15)-1
	e := &k.t.Emblems[s.embIdx(p, i)-1]
	if e.CapturePlayer != kop.RefNone {
		return false // a player capture has no object statics
	}
	if e.Capture == kop.RefNone {
		return true
	}
	return int(s.embNum(p, i)) == int(s.perms[sl].obj)+1
}

// canAtkDefGen is staticFlagGen for StatCanAttackDefender: the grant of a
// capturing emblem (The Pride of Hull Clade's "can attack as though it
// didn't have defender") reaches only the captured creature.
//
//go:noinline
func (s *State) canAtkDefGen(k *prep, sl Slot) bool {
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCanAttackDefender && s.statCond(k, st) &&
			(st.emb == 0 || s.embCaptureOK(k, st.emb, sl)) && s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull) {
			hit = true
			return false
		}
		return true
	})
	return hit
}

// embCaptureLapse zeroes the capture of every CaptureLapse emblem whose
// captured object is o, which just left the battlefield: gorge forgets a
// remembered card the moment it moves (effectMoveSweep), so a returned
// object (CR 400.7) is not the captured one.
//
//go:noinline
func (s *State) embCaptureLapse(k *prep, o Obj) {
	for p := uint8(0); p < s.nSeats(); p++ {
		if s.out(p) {
			continue
		}
		for i := 0; i < int(s.g.nemb[p]); i++ {
			e := &k.t.Emblems[s.embIdx(p, i)-1]
			if e.CapturePlayer != kop.RefNone {
				continue // a player capture is a seat, not an object
			}
			if e.CaptureLapse {
				s.embRemDrop(p, i, o) // every slot naming o (static_emblemrem.go)
			}
		}
	}
}
