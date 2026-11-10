package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtCopyPicked: CopyPermanent Choices$ (gorge effects/copypermanent.go's
// supportsChoice arm): the controller's mandatory pick, made by the OpChooseCard
// before it, is copied. With nothing to pick gorge makes no copy but still
// registers the AtEOT$ rider's delayed trigger (scheduleAtEOTNone), which
// goes on the stack at the end step and does nothing (CR 603.7).

// copyPicked is ExtCopyPicked. True when a copy's ask parked the resolution
// (the shapes that would pose one stall instead, as copyNamed does).
func (s *State) copyPicked(k *prep, in *kop.Instr) bool {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		if in.B.N&(kop.CpSacAtEndDly|kop.CpExileAtEnd) != 0 {
			s.delayEmpty(&s.res.e)
		}
		return false
	}
	c := s.objs[o].card
	if s.objs[o].zone == kop.ZoneBattlefield {
		c = s.cardAt(s.objs[o].slot)
	}
	cin := kop.Instr{Op: kop.OpCopyAsTokenEx, Obj: in.Obj, A: kop.Const(1), B: in.B, C: in.C}
	if k.cards[c].types&kop.TAura != 0 && k.t.Cards[c].Enchant.Kind == kop.TgtObject || s.enterChoiceKind(k, c) != 0 ||
		k.gen != nil && k.gen.anyBecomeCopy && s.cloneTokenAsk(k, &cin, c) != 0 {
		s.stall(StallCopyAllAsk)
		return false
	}
	return s.copyCardToken(k, &cin, c)
}
