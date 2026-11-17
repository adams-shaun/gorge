package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// windowSymbolled is the colours of sl's "Choose a mana ability" ask that
// carry a mana symbol, and whether the view mixes them with ability options
// that carry none. A printed "Produced$ Combo" ability is flattened into one
// option per colour, each with its symbol; a granted basic land type's
// intrinsic ability (Urborg's {B} on a Value Town) is a plain labelled option
// with no symbol (rules/mana_activation.go activateManaFor). gorge's answer
// (payexec.WindowAnswer) takes the option whose symbol an owed colour matches,
// else the lowest symbolled colour, and an option without a symbol only when
// no option has one, so a colour the symbol-less ability alone makes is never
// answered beside a printed Combo. ok is false for a view whose options are
// all symbol-less (a dual land's intrinsic abilities, a Verge): its answer is
// the first option listed, the owed ability first (owedAbilitiesFirst).
func (s *State) windowSymbolled(k *prep, sl Slot, pc *pcard) (mask uint8, ok bool) {
	if k.gen == nil || pc.blgFirst == 0 || pc.manaAb < 0 {
		return 0, false
	}
	ab := manaAbView(&k.t.Cards[s.cardAt(sl)], pc)
	if ab == nil || ab.AFlags&kop.AfManaChoice == 0 || ab.ManaColors == kop.ColAny {
		return 0, false
	}
	mask = uint8(ab.ManaColors) & 0x1f
	return mask, mask != 0 && uint8(pc.manaCol)&0x1f&^mask != 0
}

// windowAbilityAskCol is the colour a window source answers when its ability
// ask lists what the cost owes first and no announced pip rides the ask: the
// first owed colour of the options gorge's answer reads (windowSymbolled).
func (s *State) windowAbilityAskCol(k *prep, p uint8, useful *kop.ManaCost, sl Slot, pc *pcard) int {
	if mask, ok := s.windowSymbolled(k, sl, pc); ok {
		u := *useful
		for c := range u.Pips {
			if c >= 5 || mask&(1<<c) == 0 {
				u.Pips[c] = 0 // a pip only a symbol-less option makes is not matched
			}
		}
		useful = &u
	}
	return s.windowCol(k, p, useful, sl, pc, false, false)
}
