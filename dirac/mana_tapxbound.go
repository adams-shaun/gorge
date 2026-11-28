package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// gorge's bounded necessary-condition proof (rules potential_plan.go
// potentialPlayVerdictShared, census.boundedProof): on a board whose census
// is unrelaxable only because a repeatable tap-another source is live (Relic
// of Legends' "Tap an untapped legendary creature you control: Add one mana of
// any color", Baylen's two tokens), gorge still counts the play's search slot
// but skips the search when no plan pays the play with each such source
// relaxed to one free any-colour unit per activation its untapped candidates
// can pay for (tapRepeatBound: candidates / N). A mana payment taps and never
// untaps (CR 601.2g, 605.3a), so a play that bound cannot pay is not offered,
// even where the kernel's search would reach it by passing priority and
// letting an untap trigger resolve (Captain America, Living Legend untapping
// the legendary creature Relic tapped).
//
// The kernel mirrors it with the gaps walk under turnTapXBound: availTapX adds each
// repeatable source's unit tapXBoundRepeats times. It runs only on boards
// whose census gaps are tap-another sources alone, where every gap has a
// bounded relaxation (census.bounded); beside a sacrifice-any source, a
// repeatable filter or any other gap kind it stays off.

// tapXBoundWalk fills out with the plays the bounded walk lists and returns
// it, or nil when the board is not one the bound reads (the search then runs
// for every walk play, as before). sac: a sacrifice-any source makes the
// board unrelaxable (scriptOffers' sacAny, sacOpen, sacRepeat).
func (s *State) tapXBoundWalk(k *prep, p uint8, rt *scriptRoute, sac bool, out *Decision) *Decision {
	if !rt.twoLive || sac || rt.repeatLive || !k.tapMana || !s.tapXTwoLive(k, p) || !s.tapXOnlyGaps(k, p, rt) {
		return nil
	}
	s.g.turnF |= turnTapXBound
	s.plainOptions(k, out, availGaps)
	s.g.turnF &^= turnTapXBound
	return out
}

// turnTapXBound is genState.turnF's bit 13: the gaps walk is tapXBoundWalk's
// (tapXBoundRepeats). Like turnUnlessWin it lives in turnF's free bits, set
// and cleared around the walk, so State's fixed layout does not move.
const turnTapXBound turnFT = 1 << 13

// tapXOnlyGaps: p's census gaps are tap-another sources alone, none of them a
// sacrifice or graveyard-exile one (rt holds scriptOffers' gap masks).
func (s *State) tapXOnlyGaps(k *prep, p uint8, rt *scriptRoute) bool {
	if rt.dh || rt.ca || rt.mix || rt.filt.any() || rt.tapsDef.any() || rt.restr.any() || rt.anyR.any() ||
		rt.anyDiff.any() || rt.life.any() || s.scriptGaps(k, p).any() || s.scriptCtr(k, p).any() {
		return false
	}
	for b := s.scriptTapXModelled(k, p); b.any(); b = b.rest() {
		if k.tapXAbOf(s.cardAt(b.first())).Cost.Flags&(kop.CostSac|kop.CostExileGrave) != 0 {
			return false
		}
	}
	return true
}

// tapXBoundRepeats is how many units availTapX adds for tap-another source sl:
// one, or under the bounded walk (turnTapXBound) for a repeatable shape
// (no {T}, not once-only: tapXTwoLiveIn's) its untapped candidates over the
// cost's count (gorge tapRepeatBound).
func (s *State) tapXBoundRepeats(k *prep, p uint8, sl Slot, ab *kop.Ability) int {
	if s.g.turnF&turnTapXBound == 0 || ab.Cost.Flags&(kop.CostTap|kop.CostSac|kop.CostExileGrave) != 0 || kop.BoundedOnceManaAb(ab) || ab.Cost.N <= 0 {
		return 1
	}
	var cs [MaxPerm]Slot
	return s.tapXCands(k, p, sl, &cs) / int(ab.Cost.N)
}
