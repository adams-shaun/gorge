package dirac

// A paid filter whose fee is more than the mana it makes (Celestial Prism's
// "{2}, {T}: Add one mana of any color") takes mana out of the relaxed census
// run, so the run that charges every filter is not the most permissive one:
// gorge's paymentPlanRelaxProof runs the verdict for each subset of the paid
// alternatives in turn and calls a play insufficient only when every run is,
// so two Prisms over two Plains still pay {U} through one of them (CR 605.3a,
// 601.2g). availFilt charges all of p's filters, which is the whole subset's
// run; gapsWalk adds the rest.

// filtCostly: some filter of f costs more than it makes, so a smaller subset
// of f can pay what the whole set cannot.
func (s *State) filtCostly(k *prep, f slotMask) bool {
	for ; f.any(); f = f.rest() {
		if sl := f.first(); s.filtFee(k, sl) > s.filtUnits(k, sl) {
			return true
		}
	}
	return false
}

// gapsWalk fills d with the plays the relaxed proof leaves payable: the gaps
// walk with every filter of filt charged and, when one is costly, the same
// walk with each other subset of filt in turn (a filter outside the subset is
// spent on a fork, its {T} or its once-each-turn use, so no walk counts it).
func (s *State) gapsWalk(k *prep, d *Decision, filt slotMask) {
	s.plainOptions(k, d, availGaps)
	if filt.count() < 2 || !s.filtCostly(k, filt) {
		return
	}
	var sl [relaxedPaidLimit]Slot
	n := 0
	for b := filt; b.any() && n < len(sl); b = b.rest() {
		sl[n] = b.first()
		n++
	}
	for set := 0; set < 1<<n-1; set++ {
		c := s.fork()
		for i := 0; i < n; i++ {
			if set&(1<<i) == 0 {
				c.filtUse(k, sl[i])
			}
		}
		var sub Decision
		c.plainOptions(k, &sub, availGaps)
		s.unfork()
		for i := 0; i < int(sub.N) && int(d.N) < MaxOptions; i++ {
			if !hasOption(d, sub.Options[i]) {
				d.Options[d.N] = sub.Options[i]
				d.N++
			}
		}
	}
}
