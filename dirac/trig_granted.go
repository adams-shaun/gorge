package dirac

// grantTrigHosts adds to listener mask m the permanents a granting static's
// source (grantM) is attached to: a Continuous static's AddTrigger$ grant
// (gorge layer 6) makes the host a listener of the granted trigger's
// event, though its own card does not subscribe (grantedAbilitiesN then
// checks the static's filter and conditions).
//
//go:noinline
func (s *State) grantTrigHosts(k *prep, m slotMask) slotMask {
	for g := s.grantM; g.any(); g = g.rest() {
		if h := s.perms[g.first()].attached; h != NoSlot && s.occ.has(h) {
			m = m.with(h)
		}
	}
	return m
}
