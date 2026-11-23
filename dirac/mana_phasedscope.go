package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// phasedRunInScope reports whether gorge's phased prefix run (rules
// potentialPrefixScript's build(true)) can take a script for the play a on a
// board with a filter gap beside Taps-deferred ones: the run's script counts
// only for a play in phasedScope (a feeOrder run over a fee gap, a cast whose
// own cost sacrifices or returns a permanent, a hybrid-fee mana ability among
// the gaps) or past a plain run that spent its budget on a repeatable
// fee-only filter (plainLimited, phasedFallback's board). Any other play's
// phased run is out of scope (phasedOutOfScope), so a play the filter route
// found no script for stays unoffered however few taps of the deferred gaps
// alone would pay it: gorge's one search walks the filters among them and
// spends its 160 nodes first (Manabarbs over five lands beside Unknown
// Shores at 3 life: {1}{R} and {R}{R} both unoffered).
func (s *State) phasedRunInScope(k *prep, p uint8, a Action) bool {
	filt := s.scriptFilt(k, p)
	if s.filtFeeGaps(k, filt).any() || s.filtRepeatIn(k, filt) {
		return true
	}
	if a.Kind() != AkCast {
		return false
	}
	if s.gapFaceHybIn(k, filt.or(s.scriptTapsDefer(k, p))) {
		return true
	}
	o := Obj(a.Source())
	if o == NoObj || int(o) >= len(s.objs) {
		return false
	}
	return spellAddCost(k, s.objs[o].card).Flags&(kop.CostSac|kop.CostSacSelf|kop.CostReturn|kop.CostReturnSelf) != 0
}
