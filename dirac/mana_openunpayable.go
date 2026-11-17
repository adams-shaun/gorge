package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// openCensusUnpayable: the activation being built costs more than its plain
// payment plan can fund, and the window census is incomplete (a sacrifice-
// another source with nothing to sacrifice, Bolg's Company alone, still
// counts as a census gap). A target menu keeps such a candidate on offer
// (abilityAffordableTargets, gorge's census fallback, CR 601.2g), so the
// choice reaches the payment: gorge's planner declines it and the manual
// window runs dry, aborting the activation, where the planner would panic
// "autopay found no plan". A complete census leaves the planner to its
// panic: there the offer was the defect.
func (s *State) openCensusUnpayable(k *prep, e *stackEntry, cost *kop.ManaCost) bool {
	if s.windowCensusComplete(k, e) {
		return false
	}
	chk := planner{dry: true, noGrant: true}
	s.tapForPK(k, e.ctrl, cost, int(e.x), NoObj, false, false, &chk)
	return !chk.found
}
