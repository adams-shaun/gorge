package dirac

// resolveSpecialTapTriggers resolves the triggers a hand special action's
// source taps queued (a Treasure sacrificed beside Bloodroot Apothecary,
// CR 605.3b, 603.3b). gorge prices the special actions (plot, foretell,
// suspend) by a witness whose plan activates each source at priority, so a
// sacrifice trigger is placed and resolves, the other seats passing, before
// the action is taken; the mana it paid for floats through the resolution
// (CR 500.5 empties it only at a step's end). Like the sorcery-speed play's
// prefix (resolvePrefixStack): the kernel's single-pass payment would
// otherwise leave the trigger waiting above the action.
func (s *State) resolveSpecialTapTriggers(k *prep) {
	if s.npend == 0 || s.over {
		return
	}
	s.resolvePrefixStack(k, s.sp, true)
}
