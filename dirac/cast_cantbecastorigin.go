package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// matchSpellCardOffer is matchSpellCard for a CantBeCast static's ValidCard$
// read at a cast OFFER: the candidate (object o) is still in its hand,
// exile or graveyard, not on the stack, so a cast-provenance clause
// (PredWasCastSpell: Avatar's Wrath, Drannith Magistrate's
// Card.!wasCastFromTheirHand) cannot find a stack entry. It reads the
// origin the cast would come from instead -- the object's current zone,
// restrProvHolds (the mana-restriction read, cast_wascastrestrict.go) --
// and matches the rest of the alternative as matchSpellCard does. No
// object (o == NoObj, unit tests) fails closed. wasCastSpell stays the
// stack-entry reader for triggers and conditions.
func (s *State) matchSpellCardOffer(k *prep, f kop.FilterID, c CardID, owner, ctrl uint8, o Obj, src Slot) bool {
	if f == 0 {
		return true
	}
	var og restrOrigin
	if o != NoObj {
		og = s.restrOriginOfObj(o)
	}
	for f != 0 {
		fl := k.t.Filters[f]
		next := fl.Or
		prov, n := fl.Pred == kop.PredWasCastSpell, fl.PredN
		if prov {
			fl.Pred, fl.PredN = kop.PredNone, 0
		}
		if s.matchSpellAlt(k, fl, c, owner, ctrl, o, src) && (!prov || restrProvHolds(n, og)) {
			return true
		}
		f = next
	}
	return false
}
