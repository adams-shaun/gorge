package dirac

// markFogFirst records that the first-strike damage step passed for every
// creature that deals damage in it, when a prevention effect skipped the
// damage pass (combatDamage sets pfDealtFirst as it goes). CR 510.4: a
// creature with first strike and not double strike deals no regular combat
// damage, however its first-strike damage fared; gorge reads the keyword, not
// a record, so the kernel must not pose that creature's division again.
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
		if bp := &s.perms[b]; bp.flags&pfBlocking != 0 && bp.blocking != NoSlot && s.dealsNow(k, b, true) {
			bp.flags |= pfDealtFirst
			s.touchSl(b)
		}
	}
}
