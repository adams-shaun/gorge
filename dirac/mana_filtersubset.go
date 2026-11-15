package dirac

// The relaxed proof's paid alternatives are taken a subset at a time (gorge
// rules/potential_plan.go paymentPlanRelaxProof: 2^n runs, each with its
// subset's fees charged as generic). One run with every paid filter is the most
// permissive only while each filter makes at least what its fee takes: Henge of
// Ramos's "{2},{T}: one mana of any color" makes one mana for two, so beside a
// Simic Signet the run that includes it is short of mana that the run without
// it pays (CR 605.3a, 601.2g-h). A filter making fewer mana than its fee costs is
// therefore toggled by availFiltSkip's bits: the walk runs once per subset of the
// first two such filters (in slot order) and the plays are united. A third such
// filter stays in every run.

// availFiltSkip: with availGaps, the bits (32, 64) leave out the first and the
// second filter, in slot order, that makes fewer mana than its fee.
const availFiltSkip uint8 = 32 | 64

// filtNetNegative is how many of the filters f make fewer mana than their fee
// takes, at most two (the number availFiltSkip toggles).
func (s *State) filtNetNegative(k *prep, f slotMask) int {
	n := 0
	for ; f.any() && n < 2; f = f.rest() {
		if sl := f.first(); s.filtUnits(k, sl) < s.filtFee(k, sl) {
			n++
		}
	}
	return n
}

// plainGapsOptions is the gaps walk united over the subsets of f's net-negative
// filters (availFiltSkip).
func (s *State) plainGapsOptions(k *prep, d *Decision, f slotMask) {
	s.plainOptions(k, d, availGaps)
	n := s.filtNetNegative(k, f)
	for m := 1; m < 1<<n; m++ {
		var w Decision
		s.plainOptions(k, &w, availGaps|uint8(m)<<5)
		for i := 0; i < int(w.N); i++ {
			if !hasOption(d, w.Options[i]) {
				d.add(w.Options[i])
			}
		}
	}
}
