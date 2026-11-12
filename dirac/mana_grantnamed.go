package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// stepAbility is the printed or granted ability a prefix step on sl taps, nil
// when the step's view names none (a grant/var/gate view's index names another
// card's ability; mana_window.go windowCol). gi is the granted ability the step
// taps, -1 for the source's own.
func (s *State) stepAbility(k *prep, sl Slot, gi int16, pc *pcard) *kop.Ability {
	cd := s.cardAt(sl)
	if gi < 0 {
		if ra := s.ownRestrAb(k, sl); ra != nil && !s.ownManaUsable(k, sl) {
			return ra // the printed restricted ability (mana_grantrestrictedown.go)
		}
		if pc.manaAb >= 0 && (pc == &k.cards[cd] || pc.blgFirst != 0) {
			return manaAbView(&k.t.Cards[cd], pc)
		}
		return nil
	}
	if r := k.gen.grantOf[gi]; !k.gen.grantSelf[gi] {
		return &k.t.Cards[r.card].Abilities[r.ab]
	}
	return nil
}

// stepNamed reports whether the answers a prefix step on sl walks name their
// colour in gorge's "choose a mana ability" ask. Only a Combo ability of
// several colours is flattened into one option per colour with Option.ManaSymbol
// (pay.ManaAbilityComboColours); a plain fixed-colour ability, one of the
// several a dual land's land types give (Breeding Pool's {U} and {G}), or a
// granted basic type's intrinsic one is a single option without a symbol, which
// scriptOrderAnswers never moves ahead for a colour the play needs. mask is the
// colours the step walks. An unknown ability keeps the colour ordering.
func (s *State) stepNamed(k *prep, sl Slot, gi int16, mask uint8) bool {
	if mask&uint8(kop.ColAny) == uint8(kop.ColAny) {
		return true // an any-colour ability's colour ask names every colour
	}
	var pc *pcard
	if gi < 0 {
		pc = s.manaPCGen(k, sl)
	}
	ab := s.stepAbility(k, sl, gi, pc)
	if ab == nil {
		return true
	}
	if ab.AFlags&kop.AfManaChoice == 0 || bits.OnesCount8(uint8(ab.ManaColors)&0x3f) < 2 {
		return false
	}
	return mask&^uint8(ab.ManaColors) == 0 // colours beyond the Combo's are the intrinsic abilities' (CR 305.7)
}
