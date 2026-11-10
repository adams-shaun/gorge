package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// exileStackSelf exiles the resolving spell from the stack as the second
// half of an unless cost (ExileFromStack<1/Card.Self>; Sphinx's Approach,
// kop.PayBSubStackSelf). The entry stays on the stack until the resolution
// finishes, but its object is already in exile, so finishResolve moves
// nothing: finishing the spell must not put its card in a graveyard too.
//
//go:noinline
func (s *State) exileStackSelf(k *prep) {
	e := &s.res.e
	if s.objs[e.obj].zone != kop.ZoneStack {
		return
	}
	s.touch(e.obj)
	s.objs[e.obj].zone = kop.ZoneNone
	s.placeFromK(k, e.obj, kop.ZoneExile, false, kop.ZoneStack, false)
}
