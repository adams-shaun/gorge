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
// minted two or more copies (TestDefvalidLockstep's minimum), atomically:
// a test's seeds may play side by side.
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
// as-enters ask parks the sweep (copy_allpark.go); gorge's zone batch is
// carried across the parked ask (its Ctx clone), so the copies before and
// after the answer share one batch: their enters checks are held across the
// answer (genState.held, copy_allpark.go) and fire, in entry order, when
// the sweep ends (fireHeld). True when a copy's as-enters ask parked the
// sweep (the interpreter halts at the OpExt, execExt).
func (s *State) copyAllEach(k *prep, in *kop.Instr) bool {
	var ord [MaxPerm]Slot
	n := s.sweepHits(k, in.F, &ord)
	ctrl := s.res.e.ctrl
	var cnt [MaxPerm]int16
	var rwCard CardID
	rwN, rewritten := 0, false
	for i := 0; i < n; i++ {
		tc, c, rw, ok := s.copyTokenRewrite(k, ctrl, s.cardAt(ord[i]), 1, false)
		if !ok {
			return false // StallTokRewrite
		}
		if rw {
			rwCard, rwN, rewritten = tc, rwN+c, true
		}
		cnt[i] = int16(c)
	}
	if rewritten {
		// a CreateToken rewriter replaced every copy with a plain mint of its
		// card: no copy is left for the sweep's riders (repl_tokrewrite.go)
		s.mintRewritten(k, ctrl, rwCard, rwN)
		return false
	}
	if DebugDefValidFirstOnly && n > 1 {
		n = 1
	}
	// the copies of a run enter together (CR 603.6a): each one's "another
	// creature enters" sees the others, as gorge's zone batch does
	var eb entBatch
	stamp := s.ts
	w := &s.g.sweep
	w.n = uint8(n)
	seg := 0
	parked := s.sweepRun(k, in, w, &eb, &seg, &ord, &cnt, 0, 0)
	if parked && s.g.stalled == 0 {
		w.ord, w.cnt, w.minted = ord, cnt, int16(seg)
		// the sweep's enters checks wait for its end (fireHeld in
		// sweepResume, copy_allpark.go): a later copy's "another creature
		// enters" matches the earlier entries too (CR 603.6a)
		s.g.held, s.g.retTS = 0, stamp
		s.holdBatch(&eb)
		return true
	}
	s.defValidMulti(seg)
	s.fireEntries(k, &eb)
	return false
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
