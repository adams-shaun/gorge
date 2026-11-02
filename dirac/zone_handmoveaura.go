package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// a hand-to-battlefield move of one card whose filter may
// admit an Aura (CR 303.4f/g). gorge reaches the one gate every non-cast Aura
// entry shares (rules.auraEntryGate) from the hand walk's settleHandMove:
// with no legal object or player the Aura stays in the hand, with one it
// enters attached to it, with two or more its controller chooses as it
// enters (the "enchant" election, answered before the move folds).

// DebugHandAuraNoAttach enters such an Aura unattached (TestHandauraMutation):
// the state-based actions put it into the graveyard, where gorge has it
// attached.
var DebugHandAuraNoAttach bool

// handAuraEnter decides the picked card o's entry under ctrl. A card that is
// no Aura is none of this (host NoSlot, neither parked nor stay). stay: the
// Aura has no legal host and remains in the hand. parked: two or more hosts,
// the card waits in the hand and the resolution suspends at the raGenEnchant
// ask; its answer re-enters moveFoundAns with host answered. Otherwise host
// is what the Aura enters attached to (NoSlot for a non-Aura).
//
//go:noinline
func (s *State) handAuraEnter(k *prep, o Obj, ctrl uint8, answered Slot) (host Slot, parked, stay bool) {
	if answered == NoSlot {
		var ask bool
		if host, ask, stay = s.auraEntry(k, o, ctrl); ask || stay {
			return NoSlot, ask, stay
		}
	} else {
		host = answered
	}
	if DebugHandAuraNoAttach {
		host = NoSlot
	}
	return host, false, false
}

// handAuraArmed reports whether the running instruction is the one-card hand
// move marked SrAuraEntry; false when moveFound is driven outside a program
// (kernel tests).
//
//go:noinline
func (s *State) handAuraArmed(k *prep) bool {
	pr := s.prog(k)
	if !s.res.active || int(s.res.pc) >= len(pr) {
		return false
	}
	in := &pr[s.res.pc]
	return in.Op == kop.OpChooseMove && in.B.N&kop.SrAuraEntry != 0
}
