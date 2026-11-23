package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// consumedAuraHost reports the mana source an Aura target hangs on that its
// own payment removes: gorge's search plays the script's taps and then asks
// the cast for a legal target (potential_script.go targetsGone, castTargetsAvailable),
// and an Aura on a source that is sacrificed to pay falls off as a
// state-based action (CR 704.5m), so the target is stranded by every plan that
// uses that source. The consuming source is a depletion rider on its last counter (Sandstone
// Needle's "{T}, Remove a depletion counter: Add {R}{R}", then "if there are
// no depletion counters, sacrifice it", CR 605.3b), whose first activation
// takes the counter that triggers its sacrifice. A depletion source with a
// counter to spare, or a plain {T} unit beside the rider, consumes nothing.
func (s *State) consumedAuraHost(k *prep, p uint8, t Obj) (Slot, bool) {
	sl := s.objs[t].slot
	if !s.occ.has(sl) {
		return NoSlot, false
	}
	if s.typesGen(k, sl)&kop.TAura == 0 {
		return NoSlot, false // an Equipment only unattaches when its bearer goes (CR 704.5n)
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
	// A tap-and-sacrifice or sacrifice-only host is no stranding: the plain
	// witness announces the target before the mana is paid (CR 601.2c-h) and
	// gorge's witnessAnnouncesBeforePaying keeps every candidate the plan did not
	// itself sacrifice, the Aura the host bears among them (the state-based
	// action that drops it, CR 704.5m, waits for the cast to finish): Excavation
	// Technique over Unquestioned Authority on a Catalyst Elemental sacrificed
	// for {R}{R}. Only the depletion rider above, a script's own sacrifice,
	// strands it.
	return host, false
}
