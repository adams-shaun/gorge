package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// consumedAuraHost reports the mana source an Aura target hangs on that its
// own payment removes: gorge's search plays the script's taps and then asks
// the cast for a legal target (potential_script.go targetsGone, castTargetsAvailable),
// and an Aura on a source that is sacrificed to pay falls off as a
// state-based action (CR 704.5m), so the target is stranded by every plan that
// uses that source. The consuming sources are the tap-and-sacrifice and the
// sacrifice-only units, and a depletion rider on its last counter (Sandstone
// Needle's "{T}, Remove a depletion counter: Add {R}{R}", then "if there are
// no depletion counters, sacrifice it", CR 605.3b), whose first activation
// takes the counter that triggers its sacrifice. A depletion source with a
// counter to spare, or a plain {T} unit beside the rider, consumes nothing.
func (s *State) consumedAuraHost(k *prep, p uint8, t Obj) (Slot, bool) {
	sl := s.objs[t].slot
	if !s.occ.has(sl) {
		return NoSlot, false
	}
	host := s.perms[sl].attached
	if host == NoSlot || host == sl || !s.occ.has(host) || !s.ctrlM[p].has(host) {
		return NoSlot, false
	}
	if k.gen != nil && k.gen.anyManaRider && s.depleteCensusGap(k, p, host) {
		a, ok := s.riderAbilityOf(k, host)
		return host, ok && a.ManaRider == kop.ManaRiderDeplete &&
			s.perms[host].ctrN(s, int(a.Cost.Ctr)) == int(a.Cost.N)
	}
	m := s.g.availX
	s.g.availX = availPlain
	pc := s.manaPCGen(k, host)
	s.g.availX = m
	return host, pc.manaSac || pc.manaSacOnly
}
