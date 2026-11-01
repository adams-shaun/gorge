package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// non-token transforming double-faced cards. The
// transform itself is OpTransform (act_transform.go); this file adds
// the two entry routes gorge has around it: a ChangeZone Transformed$ True
// return enters the back face (CR 712.14a, effects zone_change.go
// applyTransformed), and a Mode$ Transformed trigger follows every
// battlefield flip (CR 701.27a, rules trigmatch transformed.go).

// FaceTransformStats counts the lifted mechanics; oracle tests only.
type FaceTransformStats struct {
	Enters int // MvTransformed entries (enterTransformed turned a face)
	Checks int // fireTransformed reached the battlefield scan
	Fires  int // TrigTransformed triggers actually queued
	PredMV int // a CMC predicate read a back face
}

// DebugFaceTransformStats is the test hook; nil in normal play.
var DebugFaceTransformStats *FaceTransformStats

// DebugIgnoreTransformed disables the ChangeZone Transformed$ entry (the
// oracle mutation test); false in normal play.
var DebugIgnoreTransformed bool

// enterTransformed turns an object entering the battlefield with
// MvTransformed to face 1 before auraEntry reads it, as gorge's
// applyTransformed emits the FlipFace before the MoveZone. A card off the
// battlefield is always front face up, so this equals (FaceIdx+1)%2; the
// face test keeps it idempotent if auraEntry asks and re-enters execReturn.
//
//go:noinline
func (s *State) enterTransformed(k *prep, o Obj) {
	if DebugIgnoreTransformed {
		return
	}
	ob := &s.objs[o]
	cd := &k.t.Cards[ob.card]
	if cd.FaceMode != kop.FaceTransform || cd.Face != 0 || len(cd.Faces) < 2 {
		return
	}
	s.turnFace(k, o, 1)
	if DebugFaceTransformStats != nil {
		DebugFaceTransformStats.Enters++
	}
}

// fireTransformed fires "when <filter> transforms" (gorge Mode$
// Transformed). No slot subscription: scan the battlefield as for a gift or
// a monstrosity. Called after the face's subscriptions are restored, so a
// back face's own printed trigger is seen. The entering-transformed flip is
// off the battlefield and never reaches this path.
//
//go:noinline
func (s *State) fireTransformed(k *prep, sl Slot, o Obj, p uint8) {
	if k.gen == nil || !k.gen.anyTransformed {
		return
	}
	if DebugFaceTransformStats != nil {
		DebugFaceTransformStats.Checks++
	}
	var ord [MaxPerm]Slot
	n := s.listenerOrder(s.occ, &ord)
	before := s.npend
	for _, ls := range ord[:n] {
		s.slotTriggers(k, ls, func(a *kop.Ability) bool { return a.Trig.Event == kop.TrigTransformed }, func(l trigListener, a *kop.Ability) {
			s.objTrigGen(k, kop.TrigTransformed, l, a, sl, o, p, 0)
		})
	}
	if DebugFaceTransformStats != nil {
		DebugFaceTransformStats.Fires += int(s.npend - before)
	}
}
