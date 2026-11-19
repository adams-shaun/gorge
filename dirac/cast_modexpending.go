package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// modeTargetFeasible is hasTarget for one target of a mode the cast is still
// announcing. CR 601.2b announces the modes before the value of {X}, so a mode
// whose target an X bounds (Confront the Past's "planeswalker card with mana
// value X or less") is judged with that X pending, as the offer's census does
// (xBoundPending): no X makes a target of what the rest of the filter rules
// out, and any other object may fit some X. gorge's castModeAsk reads the same
// pending X (modeTargetsAvailable with !pc.xDone) and the X menu that follows
// keeps only the values the chosen mode's target admits (addXOptions).
func (s *State) modeTargetFeasible(k *prep, t *kop.TargetSpec) bool {
	e := &s.bld().e
	if e.kind != seSpell || s.bld().stage&^0x80 != bsModes || k.t.Cards[e.card].Cost.X == 0 || !xBoundTarget(k.t, t) {
		return s.hasTarget(k, t, e.ctrl, s.srcSlot(), e.obj)
	}
	saved := s.g.probe
	s.g.probe = probe{kind: probeSpell, src: NoSlot, card: e.card, ctrl: e.ctrl}
	ok := s.hasTarget(k, t, e.ctrl, s.srcSlot(), e.obj)
	s.g.probe = saved
	return ok
}
