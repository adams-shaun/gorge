package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// "Target creature blocks this turn if able." (CR 509.1c; gorge Effect
// StaticAbilities$ Mode$ MustBlock | ValidCreature$ Card.IsRemembered over
// RememberObjects$ Targeted, ExileOnMoved$ Battlefield, read by
// combat.MustBlockCandidates in askBlockers): a BLOCKER requirement. gorge
// marks every offered pair of such a blocker Required (BlockMust) and its
// blockRequiredCore counts one unit per required blocker that blocks; the
// kernel adds the same unit to the maximum-requirements search of
// combat_mustbeblocked.go (mbSearch.duty), so the defender is offered a pair
// only while the maximum stays reachable and Done only once it is met.
//
// ExtBlocksIfAble marks the target's slot in the arena bit mechBlocksIfAble
// (0 B): cleared as the next turn begins (the effect lasts this turn) and
// when a new permanent takes the slot (arenaClrSlot: the effect ends as the
// creature leaves, ExileOnMoved$ Battlefield, CR 400.7). The duty sits on
// the Effect, not on the creature, so losing its abilities keeps it.

// blocksIfAbleExt is ExtBlocksIfAble: the battlefield object Obj is marked.
func (s *State) blocksIfAbleExt(k *prep, in *kop.Instr) {
	if sl := s.objRef(k, in.Obj); sl != NoSlot && !s.mSl(k, mechBlocksIfAble, sl) {
		s.mSlSet(k, mechBlocksIfAble, sl)
		s.touchSl(sl)
	}
}

// blockDuty is the defending player's permanents that block this turn if
// able (empty when the table never marks one).
func (s *State) blockDuty(k *prep) slotMask {
	return s.mSlMask(k, mechBlocksIfAble).and(s.ctrlM[s.blockDef()])
}
