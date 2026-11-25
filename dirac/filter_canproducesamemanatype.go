package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// fwCanProduceSameManaTypeWith: FwCanProduceSameManaTypeWith, the candidate
// battlefield permanent shares at least one producible type of mana with an
// object the referent names (gorge's canProduceSameManaTypeWith over
// SpecContext.TriggerCard, engine/effects/filter_word_manatype.go; Mana Web's
// TapAll body). The producible set is the printed mana abilities' types:
// CardDef.ProduceCol for WUBRG and {C} when a printed mana ability makes it,
// the same census producesColourless reads for a reflected source. An
// unbound referent, a candidate that produces nothing, or a referent that
// produces nothing matches nothing, fail closed.
//
//kmanifest:fword FwCanProduceSameManaTypeWith
func (s *State) fwCanProduceSameManaTypeWith(c *fctx, l *kop.FLit) bool {
	if !c.perm() {
		return false
	}
	cand := s.cardProducible(c.k, s.cardAt(c.sl))
	if cand == 0 {
		return false
	}
	for _, r := range s.fwReferentObjs(c, l.R) {
		if cand&s.objProducible(c.k, r) != 0 {
			return true
		}
	}
	return false
}

// cardProducible is the types of mana one card's printed mana abilities could
// produce: CardDef.ProduceCol (WUBRG, {C} dropped) plus {C} when a printed
// mana ability makes it. A reflected or latent mana ability contributes no
// colourless of its own (producesColourless' exclusion: it reads its
// reflected source instead), exactly as gorge's producibleSymbols over
// Face.ManaAbilities. A card with no printed mana ability produces nothing.
func (s *State) cardProducible(k *prep, c CardID) kop.ColorMask {
	if k == nil || int(c) >= len(k.t.Cards) {
		return 0
	}
	cd := &k.t.Cards[c]
	m := cd.ProduceCol & kop.ColAny
	for i := range cd.Abilities {
		a := &cd.Abilities[i]
		if a.Kind == kop.AbMana && a.AFlags&(kop.AfLatent|kop.AfManaReflect) == 0 && a.ManaColors&kop.ColC != 0 {
			m |= kop.ColC
			break
		}
	}
	return m
}

// objProducible is cardProducible for an object: its battlefield slot's card
// (a copy override included) while it is on the battlefield, its card
// otherwise (CR 400.7: a departed object still names the card whose printed
// abilities the predicate reads).
func (s *State) objProducible(k *prep, o Obj) kop.ColorMask {
	if int(o) >= len(s.objs) {
		return 0
	}
	ob := &s.objs[o]
	if ob.zone == kop.ZoneBattlefield && ob.slot != NoSlot {
		return s.cardProducible(k, s.cardAt(ob.slot))
	}
	return s.cardProducible(k, ob.card)
}
