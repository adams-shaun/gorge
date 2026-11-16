package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// spellTargetsFeasibleHand: a spell cast from the hand has left it for the
// stack by the time its targets are chosen (CR 601.2a, 601.2c), so a target
// whose characteristics count the cards in a hand is judged without the spell:
// Sturmgeist ("power and toughness are each equal to the number of cards in
// your hand") is a 4/4 with four cards in hand and no target for Sungold
// Barrage ("toughness 4 or greater") once the Barrage itself is cast
// (gorge offerAsSpellOnStack probes the target census the same way).
//
// Only a characteristic-defining P/T on the battlefield (gfCDAPT) reads the
// hand this way, so every other offer keeps the plain check; the probe is a
// fork with the card on the stack, as planHoldsOnStack's second fork.
func (s *State) spellTargetsFeasibleHand(k *prep, c CardID, p uint8, o Obj) bool {
	if k.gen == nil || !s.battlefieldHasCDAPT(k) {
		return s.spellTargetsFeasible(k, c, p)
	}
	u := s.fork()
	defer s.unfork()
	u.detach(o)
	u.objs[o].zone = kop.ZoneStack
	return u.spellTargetsFeasible(k, c, p)
}

// battlefieldHasCDAPT: some permanent carries a characteristic-defining P/T.
func (s *State) battlefieldHasCDAPT(k *prep) bool {
	for seat := range s.ctrlM {
		for m := s.ctrlM[seat]; m.any(); m = m.rest() {
			if k.gen.flags[s.cardAt(m.first())]&gfCDAPT != 0 {
				return true
			}
		}
	}
	return false
}
