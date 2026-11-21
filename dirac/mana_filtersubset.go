package dirac

// The relaxed proof's paid alternatives are taken a subset at a time (gorge
// rules/potential_plan.go paymentPlanRelaxProof: 2^n runs, each with its
// subset's fees charged as generic). One run with every paid filter is the most
// permissive only while each filter makes at least what its fee takes: Henge of
// Ramos's "{2},{T}: one mana of any color" makes one mana for two, so beside a
// Simic Signet the run that includes it is short of mana that the run without
// it pays (CR 605.3a, 601.2g-h). A filter making fewer mana than its fee costs is
// therefore toggled by filtSkip: the walk runs once per subset of the first
// relaxedPaidLimit such filters (in slot order) and the plays are united.

// filtSkip is the gaps walk's subset of net-negative filters to leave out,
// bit i the i-th such filter in slot order; 0 outside the walk. It rides one
// byte of genState.rsv (the reserved tail for new State bytes), so it aliases
// no availFor mode bit and State keeps its pinned layout.
func (s *State) filtSkip() uint8 { return s.g.rsv[0] }

func (s *State) setFiltSkip(v uint8) { s.g.rsv[0] = v }

// filtNetNegative is how many of the filters f make fewer mana than their fee
// takes, at most relaxedPaidLimit (gorge's own cap on the relaxed proof's
// paid subset).
func (s *State) filtNetNegative(k *prep, f slotMask) int {
	n := 0
	for ; f.any() && n < relaxedPaidLimit; f = f.rest() {
		if sl := f.first(); s.filtUnits(k, sl) < s.filtFee(k, sl) {
			n++
		}
	}
	return n
}

// plainGapsOptions is the gaps walk united over the subsets of f's net-negative
// filters (filtSkip). m carries the walk's extra availFor mode beside availGaps.
func (s *State) plainGapsOptions(k *prep, d *Decision, f slotMask, m uint16) {
	s.setFiltSkip(0)
	s.plainOptions(k, d, availGaps|m)
	n := s.filtNetNegative(k, f)
	for mm := 1; mm < 1<<n; mm++ {
		s.setFiltSkip(uint8(mm))
		var w Decision
		s.plainOptions(k, &w, availGaps|m)
		for i := 0; i < int(w.N); i++ {
			if !hasOption(d, w.Options[i]) {
				d.add(w.Options[i])
			}
		}
	}
	s.setFiltSkip(0)
}
