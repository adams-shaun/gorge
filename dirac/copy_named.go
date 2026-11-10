package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtCopyNamed: CopyPermanent DefinedName$ (gorge effects/copypermanent.go:
// the source is a card of the compiled universe named by the ability, not an
// object; the fold mints the token from the card's printed face). The copy
// is made for the resolving controller through the same count rewriters,
// token adders and entry as an object's copy (copyCardToken), with the op's
// flags and copy exceptions. It has no template permanent, so nothing is
// inherited from a source on the battlefield.

// copyNamed is ExtCopyNamed. True when a copy's as-enters ask parked the
// resolution; the shapes that would pose one stall instead (copyTokenCtr's
// stance), so it is false unless the stall left the resolution halted.
func (s *State) copyNamed(k *prep, in *kop.Instr) bool {
	c := CardID(in.F)
	if int(c) >= len(k.t.Cards) {
		return false
	}
	cin := kop.Instr{Op: kop.OpCopyAsTokenEx, A: kop.Const(1), B: in.B, C: in.C}
	if k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject || s.enterChoiceKind(k, c) != 0 ||
		k.gen != nil && k.gen.anyBecomeCopy && s.cloneTokenAsk(k, &cin, c) != 0 {
		s.stall(StallCopyAllAsk)
		return false
	}
	return s.copyCardToken(k, &cin, c)
}
