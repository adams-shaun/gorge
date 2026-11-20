package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's phased run in its fee order (rules/potential_script.go
// scriptSearch.feeOrder). potentialPrefixScript's second run, after the plain
// run found no script, walks a plain cast's census in a different order when
// a gap's mana ability costs mana and makes more than one unit (Baxter
// Building's "{4}, {T}: Add four mana in any combination of colors"): the
// covered funding first, then those fee-carrying gaps, then the other gaps.
// The plain order activates the gap at a pool its fee has not reached and
// spends the node budget on that dead branch, so a board of five or more
// lands beside the Building (Training Compound, Forests, an Island) runs the
// plain search out of its 160 nodes while the fee order prices the play in
// about ten (std-msh pair 172: Attuma, Atlantean Warlord).
//
// The run is a cast's alone (a plain cast with no alternative cost, face or
// permission), never beside a hybrid-fee gap (that is phasedScope's own
// order); every other play keeps the plain run's answer.

// feeGapSlot reports whether the filter at sl is a multi-unit fee gap: its
// ability costs mana (every filter's does) and makes a literal amount above
// one. A dynamic amount is no fee gap: gorge's availableAmount reads it as
// zero units (Starfleet Academy's "{5}, {T}: Add X mana of one colour").
func (fs *filtSearch) feeGapSlot(k *prep, t *State, sl Slot) bool {
	return fs.gaps.has(sl) && t.filtAb(k, sl).AFlags&kop.AfManaDynamic == 0 && t.filtUnits(k, sl) >= 2
}

// feeOrderRun reports whether play fs.a gets the fee-order run after a plain
// run that found no script: a plain cast beside a multi-unit fee gap.
func (fs *filtSearch) feeOrderRun(k *prep, s *State) bool {
	a := fs.a
	if fs.pay != nil || a.Kind() != AkCast || a.Mode()&3 != 0 || a.Mode()&(castMayPlay|castFace|castWarp) != 0 || s.filtHybIn(k, fs.gaps) {
		return false
	}
	for m := fs.gaps; m.any(); m = m.rest() {
		if fs.feeGapSlot(k, s, m.first()) {
			return true
		}
	}
	return false
}

// feeOrderSearch is the fee-order run: filtScript's plain search with its own
// budget and the phased run's pruning (no plain-run fee-stall clamp).
func (fs *filtSearch) feeOrderSearch() filtSearch {
	return filtSearch{a: fs.a, p: fs.p, gaps: fs.gaps, own: fs.own, sp0: fs.sp0, need: fs.need, useful: fs.useful,
		usefulOK: fs.usefulOK, other: fs.other, feeOnly: fs.feeOnly, fees: fs.fees, dead: fs.dead, phased: true, feeOrder: true}
}

// feeOrderSkip: in the fee order's gap passes, pass 1 walks the multi-unit
// fee gaps and pass 2 the others (gorge's scriptPhase.feeOnly).
func (fs *filtSearch) feeOrderSkip(k *prep, t *State, sl Slot, pass int) bool {
	return fs.feeOrder && fs.feeGapSlot(k, t, sl) != (pass == 1)
}
