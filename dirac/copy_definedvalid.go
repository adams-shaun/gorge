package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// CopyPermanent and Animate over Defined$ Valid <filter>
// (Multiversal Incursion; Sita Varma, Masked Racer).

// DebugDefValidFirstOnly is the mutation switch of TestDefvalidMutation:
// copyAllEach mints only for the first permanent of the set.
var DebugDefValidFirstOnly bool

// DebugDefValidNoAnimateAsk is the mutation switch of
// TestDefvalidAnimateMutation: the kernel never poses MayGuardAnimateAll's ask.
var DebugDefValidNoAnimateAsk bool

// DebugDefValidMulti, when set, counts the copyAllEach resolutions that
// minted two or more copies (TestDefvalidLockstep's minimum).
var DebugDefValidMulti *uint32

// StallCopyAllAsk: a copy of a copy that itself carries the as-enters clone
// line would be minted on the copied permanent's printed card while it waits
// for the answer (copyTokenBasis; the sweep checks its source's slot,
// copy_allpark.go). A rider that would drop counters across an ask stalls
// with it too (copyTokenCtr).
const StallCopyAllAsk = 124

func init() { stallNames[StallCopyAllAsk] = "copyallask" }

// copyAllEach is OpExt ExtCopyAllEach: gorge CopyPermanent's Defined arm
// (copypermanent.go): the set read once (copies entering during the sweep
// are never visited), every source's copy count sized through the
// CreateToken replacements first, then the copies minted source by source
// in set order, each with the op's copy exceptions (in.C). A copy with an
// as-enters ask parks the sweep (copy_allpark.go); gorge's zone batch closes
// at the park, so the copies before the answer entered under their own batch
// and the rest mint under a fresh one. True when a copy's as-enters ask
// parked the sweep (the interpreter halts at the OpExt, execExt).
func (s *State) copyAllEach(k *prep, in *kop.Instr) bool {
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // gorge: one zone batch for the copies of each run
	}
	var ord [MaxPerm]Slot
	n := s.sweepHits(k, in.F, &ord)
	ctrl := s.res.e.ctrl
	var cnt [MaxPerm]int16
	for i := 0; i < n; i++ {
		cnt[i] = int16(s.replTokens(k, ctrl, s.cardAt(ord[i]), 1))
	}
	if DebugDefValidFirstOnly && n > 1 {
		n = 1
	}
	// the copies of a run enter together (CR 603.6a): each one's "another
	// creature enters" sees the others, as gorge's zone batch does
	var eb entBatch
	defer s.fireEntries(k, &eb)
	w := &s.g.sweep
	w.n = uint8(n)
	seg := 0
	parked := s.sweepRun(k, in, w, &eb, &seg, &ord, &cnt, 0, 0)
	if !parked || s.g.stalled != 0 {
		s.defValidMulti(seg)
		return false
	}
	w.ord, w.cnt, w.minted = ord, cnt, int16(seg)
	return true
}

// animateAllAsks is the MayGuardAnimateAll guard: the next op is an
// OpOverrideAll whose set has a battlefield hit (gorge animateWouldChange:
// the grant always changes something, Power$ and Toughness$ being set).
func (s *State) animateAllAsks(k *prep, next *kop.Instr) bool {
	if DebugDefValidNoAnimateAsk {
		return false
	}
	var ord [MaxPerm]Slot
	return next.Op == kop.OpOverrideAll && s.sweepHits(k, next.F, &ord) > 0
}
