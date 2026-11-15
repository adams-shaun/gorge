package dirac

// markFogFirst records, for a first-strike damage step a Fog skips, the
// creatures that took part in it: gorge notes the first strikers as the step
// begins (actsThisDamageStep / collectFirstStrikers, CR 510.4), so a first
// striker whose damage was all prevented still sits out the regular step.
// combatDamage sets pfDealtFirst in the same two places for a step that runs.
func (s *State) markFogFirst(k *prep) {
	for am := s.ctrlM[s.active]; am.any(); am = am.rest() {
		a := am.first()
		if s.perms[a].flags&pfAttacking != 0 && s.dealsNow(k, a, true) {
			s.perms[a].flags |= pfDealtFirst
			s.touchSl(a)
		}
	}
	for bm := s.oppsM(s.active); bm.any(); bm = bm.rest() {
		b := bm.first()
		bp := &s.perms[b]
		if bp.flags&pfBlocking != 0 && bp.blocking != NoSlot && s.dealsNow(k, b, true) {
			bp.flags |= pfDealtFirst
			s.touchSl(b)
		}
	}
}
