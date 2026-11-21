package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Unattach (CR 701.3b; gorge engine/effects/unattach.go effUnattach): the
// defined object, when it is a battlefield permanent still attached to an
// object, stops being attached; anything else is a no-op (gorge's
// o.Zone != ZBattlefield || o.AttachedTo == 0 skip; a permanent attached to a
// PLAYER has no AttachedTo in gorge and is skipped the same way). The
// detach publishes the Mode$ Unattached trigger with the former bearer, as
// gorge's Unattached event does (fireUnattached, trig_unattached.go). The
// state half is unattachSelf's (kw_reconfigure.go), which Reconfigure's own
// unattach half does not publish.
//
// DebugUnattachNoDetach is the lockstep mutation: the effect resolves but the
// attachment stays, so a game that unattaches must diverge at the bearer's
// next characteristic read.
var DebugUnattachNoDetach bool

// unattachExt is ExtUnattach.
//
//kmanifest:ext ExtUnattach
func (s *State) unattachExt(k *prep, in *kop.Instr) (ok, done bool) {
	sl := s.objRef(k, in.Obj)
	if sl == NoSlot || s.perms[sl].attached == NoSlot || DebugUnattachNoDetach {
		return true, true
	}
	p := &s.perms[sl]
	h := p.attached
	if s.occ.has(h) {
		s.touchSl(h)
	}
	p.attached = NoSlot
	s.touchSl(sl)
	if k.gen.anyAegis {
		s.aegisUnlink(k, sl, h, false) // zone_exilethenattach.go
	}
	s.refreshTy(k)
	if k.gen.anyUnattach {
		s.fireUnattached(k, sl, h) // trig_unattached.go
	}
	return true, true
}
