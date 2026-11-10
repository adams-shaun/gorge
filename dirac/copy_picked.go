package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ExtCopyPicked: CopyPermanent Choices$ (gorge effects/copypermanent.go's
// supportsChoice arm): the controller's mandatory pick, made by the OpChooseCard
// before it, is copied. With nothing to pick gorge makes no copy but still
// registers the AtEOT$ rider's delayed trigger (scheduleAtEOTNone), which
// goes on the stack at the end step and does nothing (CR 603.7).

// chooseCopyPick is OpChooseCard C: the CopyPermanent Choices$ pick (gorge's
// KChoose with ResumeKind "copypermanent_choice", Min 1 and Max 1, its
// options of Kind "permanent"): the mandatory single pick OpChooseCard
// already poses, with one candidate too and none with no candidate, always
// followed by an ExtCopyPicked over RefChosen. The ask is ChGenCopyPick.
const chooseCopyPick = 8

// copyPicked is ExtCopyPicked. True when a copy's ask parked the resolution
// (the shapes that would pose one stall instead, as copyNamed does).
func (s *State) copyPicked(k *prep, in *kop.Instr) bool {
	o := s.objRefObj(in.Obj)
	if o == NoObj {
		if in.B.N&(kop.CpSacAtEndDly|kop.CpExileAtEnd) != 0 {
			if sacAtUpkeepOf(in.B.N) {
				s.delayEmptyUpkeep(&s.res.e)
			} else {
				s.delayEmpty(&s.res.e)
			}
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

// copyEnterAttacking is a copy token's TokenAttacking$ True: it enters
// attacking the controller's opponent (gorge copyAttackRider: players only,
// the attack trigger's defending player, CR 508.4), unless that player has
// lost.
func (s *State) copyEnterAttacking(k *prep, sl Slot, ctrl uint8) {
	if opp := NextSeat(ctrl); !s.pl[opp].lost {
		s.setEnterAttacker(k, sl, PlayerCode(opp))
	}
}
