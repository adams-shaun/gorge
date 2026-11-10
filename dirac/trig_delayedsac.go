package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// delayedRegObj is the object a delayed registration remembers. RefSacrificed
// is the activation cost's sacrificed object (The Guardian of Forever's "return
// that card at the next end step"), read from the entry's last-known-information
// record exactly as ConditionDefined$ Sacrificed reads it; every other ref is
// the ordinary object reference.
func (s *State) delayedRegObj(r kop.Ref) Obj {
	if r == kop.RefSacrificed {
		return s.sacrificedObj(s.ctxEntry(&s.res.e))
	}
	return s.objRefObj(r)
}
