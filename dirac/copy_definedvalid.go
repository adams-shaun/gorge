package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

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

// StallCopyAllAsk: a copy-all sweep over an Aura with an object Enchant or a
// permanent with an as-enters choice; its mint would park an ask
// (copyTokens) the sweep cannot resume without State bytes.
const StallCopyAllAsk = 124

func init() { stallNames[StallCopyAllAsk] = "copyallask" }

// copyAllEach is OpExt ExtCopyAllEach: gorge CopyPermanent's Defined arm
// (copypermanent.go): the set read once (copies entering during the sweep
// are never visited), every source's copy count sized through the
// CreateToken replacements first, then the copies minted source by source
// in set order, each with the op's copy exceptions (in.C).
func (s *State) copyAllEach(k *prep, in *kop.Instr) {
	if k.gen.anyBatch {
		defer s.batchClose(s.batchOpen()) // gorge: one zone batch for all the tokens
	}
	var ord [MaxPerm]Slot
	n := s.sweepHits(k, in.F, &ord)
	for i := 0; i < n; i++ {
		c := s.cardAt(ord[i])
		if k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject || s.enterChoiceKind(k, c) != 0 || k.gen.anyBecomeCopy && s.cloneTokenAsk(k, nil, c) != 0 {
			s.capFull(StallCopyAllAsk)
			return
		}
	}
	ctrl := s.res.e.ctrl
	var cnt [MaxPerm]int16
	for i := 0; i < n; i++ {
		cnt[i] = int16(s.replTokens(k, ctrl, s.cardAt(ord[i]), 1))
	}
	if DebugDefValidFirstOnly && n > 1 {
		n = 1
	}
	minted := 0
	defer func() {
		if DebugDefValidMulti != nil && minted >= 2 {
			*DebugDefValidMulti++
		}
	}()
	for i := 0; i < n; i++ {
		sl := ord[i]
		if !s.occ.has(sl) {
			continue
		}
		c := s.cardAt(sl)
		for j := int16(0); j < cnt[i]; j++ {
			if s.nearCapacity() {
				s.stall(StallBoard)
				return
			}
			t := s.newToken(c, ctrl)
			s.objs[t].of |= ofCopy
			s.copyTokenEnterFrom(k, in, t, NoSlot, sl, nil)
			minted++
		}
	}
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
