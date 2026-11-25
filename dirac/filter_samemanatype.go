package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwSameManaType: FwSameManaType (Mana Web's "tap all lands that player
// controls that could produce any type of mana that land could produce"),
// the candidate can produce a type of mana the trigger object can produce.
// Both read the printed mana abilities: CardDef.ProduceCol for the colours
// ({C} dropped, producibleMask) and each mana ability's ManaColors for {C}
// (as producesColourless reads it). A candidate or trigger object off the
// battlefield fails closed (objRef).
//
//kmanifest:fword FwSameManaType
func (s *State) fwSameManaType(c *fctx, l *kop.FLit) bool {
	if !c.perm() {
		return false
	}
	trig := s.objRef(c.k, kop.RefTriggerObj)
	if trig == NoSlot {
		return false
	}
	tc, tt := s.permanentProducible(c.k, trig)
	cc, ct := s.permanentProducible(c.k, c.sl)
	return tc&cc != 0 || tt && ct
}

// permanentProducible is the types of mana a battlefield permanent at sl can
// produce: its printed mana abilities' colours (CardDef.ProduceCol, {C}
// dropped) and whether any of them makes {C} (ManaColors & ColC, latent and
// reflected abilities excluded as producesColourless does).
func (s *State) permanentProducible(k *prep, sl Slot) (colors kop.ColorMask, colourless bool) {
	cd := &k.t.Cards[s.cardAt(sl)]
	colors = cd.ProduceCol & kop.ColAny
	for j := range cd.Abilities {
		a := &cd.Abilities[j]
		if a.Kind == kop.AbMana && a.AFlags&(kop.AfLatent|kop.AfManaReflect) == 0 && a.ManaColors&kop.ColC != 0 {
			return colors, true
		}
	}
	return colors, false
}
