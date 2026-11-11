package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a target slot whose TargetMin$/TargetMax$ is a value (X
// paid, a count of permanents, the trigger's amount): TargetSpec.MinV/MaxV
// under TgfBoundsV. Gorge evaluates the bound as the slot is announced
// (rules/target_legal.go resolvedTargetBoundsWithGift: the asking player,
// the source's SVars, the cast's settled X, the trigger's recorded context),
// clamps min < 0 to 1 and max < 0 to 0, honours a resolved 0, then lifts max
// to min. The slot then asks like a constant one: a zero maximum asks
// nothing, a maximum above the candidates ends in a done-only ask.

// DebugTgtBoundsMaxPlus1 is the mutation switch: the evaluated maximum is
// one too many, so a slot taking every candidate asks a done-only decision
// gorge does not.
var DebugTgtBoundsMaxPlus1 bool

// tgtBoundsEval is slot t's gorge-resolved [lo, hi] for entry e.
func (s *State) tgtBoundsEval(k *prep, t *kop.TargetSpec, e *stackEntry) (lo, hi int) {
	ctx := s.ctxEntry(e)
	ctx.bounds = true
	lo, hi = s.evalV(k, t.MinV, ctx), s.evalV(k, t.MaxV, ctx)
	if DebugTgtBoundsMaxPlus1 {
		hi++
	}
	if lo < 0 {
		lo = 1
	}
	if hi < 0 {
		hi = 0
	}
	if hi < lo {
		hi = lo
	}
	return lo, hi
}

// tgtBoundsFeasible reports whether mandatory TgfBoundsV slot t (a spell's
// "X target creatures", MinV the announced X) has as many candidates as its
// minimum, X read from the probe (gorge targetsAnnounceableAt: an X the
// target ask could not answer is not offered, CR 601.2c). A minimum of 0
// needs none.
func (s *State) tgtBoundsFeasible(k *prep, t *kop.TargetSpec, p uint8, src Slot) bool {
	lo := s.evalV(k, t.MinV, evalCtx{src: NoObj, srcSl: src, ctrl: p})
	if lo <= 0 {
		return true
	}
	var buf [MaxOptions]uint8
	return s.targetCands(k, t, p, src, NoObj, &buf) >= lo
}

// tgtBoundsV sets mt's bounds for a TgfBoundsV slot of the entry being built.
// kgen lowers a literal 0 minimum, or (on a spell) exactly the announced X,
// whose X ask offered only an X with that many candidates
// (tgtBoundsFeasible), so the slot is never short.
func (s *State) tgtBoundsV(k *prep, t *kop.TargetSpec, e *stackEntry, mt *multiTgt) {
	lo, hi := s.tgtBoundsEval(k, t, e)
	if hi > 254 {
		var buf [MaxOptions]uint8
		if s.multiCands(k, t, mt.slot, &buf) > 254 {
			s.capFull(StallTables)
		}
		lo, hi = min(lo, 254), 254
	}
	if t.Divide == kop.DivMaxV {
		hi = s.divMaxVCap(hi) // the division arrays' size (ctr_divided.go)
		lo = min(lo, hi)
	}
	mt.min, mt.max = uint8(lo), uint8(hi)
}

// tgtAsksNothing reports whether slot t of entry e is a TgfBoundsV slot whose
// evaluated maximum is 0 ("X target creatures" at X = 0): gorge poses no target
// menu for it, so the menu's affordability census (affordableTargetCandidates)
// never runs and an unpayable cast reaches its mana window instead of reversing
// at the target stage (CR 601.2c, 601.2g, 733.1).
func (s *State) tgtAsksNothing(k *prep, t *kop.TargetSpec, e *stackEntry) bool {
	if t.TFlags&kop.TgfBoundsV == 0 {
		return false
	}
	_, hi := s.tgtBoundsEval(k, t, e)
	return hi == 0
}
