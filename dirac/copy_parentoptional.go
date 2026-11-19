package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// copyParentAsks is MayGuardCopyParent: gorge poses the may-copy election of
// an Optional$ CopySpellAbility Defined$ Parent only once the copy is
// possible (effCopySpellAbility: the resolving spell is still on the stack),
// the same test copyParentSpell makes before it mints a copy.
func (s *State) copyParentAsks() bool {
	e := &s.res.e
	si := int(s.sp) - 1
	return e.kind == seSpell && si >= 0 && s.stack[si].obj == e.obj && s.objs[e.obj].zone == kop.ZoneStack
}
