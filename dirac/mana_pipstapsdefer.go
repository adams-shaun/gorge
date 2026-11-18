package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// addPipTapsDefer counts the Taps-deferred sources the CR 601.2g mana window
// can activate (a land whose tap fires a Taps watcher gorge's planner defers,
// Burning Earth beside Crumbling Necropolis) as units of a hybrid or Phyrexian
// pip ask's census. gorge's census never lists them, but announceFeasible
// prices a pip against the sources the window can activate (CR 605.3a), so the
// colour face is offered when only such a source pays it. An activation whose
// {T} cost taps the source itself cannot also tap it for mana, as
// excludeTapSource has it.
func (s *State) addPipTapsDefer(k *prep, e *stackEntry, av *avail) {
	own := NoSlot
	if e.kind == seAbility && k.t.Cards[e.card].Abilities[e.ab].Cost.Flags&kop.CostTap != 0 {
		own = s.srcSlot()
	}
	for b := s.scriptTapsDefer(k, e.ctrl); b.any(); b = b.rest() {
		if sl := b.first(); sl != own {
			s.addTapsDeferUnits(k, sl, av)
		}
	}
}
