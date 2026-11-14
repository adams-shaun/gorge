package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Cast provenance. A spell's stack entry carries the zone it
// was cast from and the creatures that convoked it (stackEntry.prov,
// recorded only for a table with a reader: genPrep.anyCastProv). The
// readers see it while the cast is still on the stack or resolving:
//
//   - Tf2CastFromHand on SpellCast: the spell was cast from its owner's
//     hand by the trigger's controller (gorge wasCastFromYourHandByYou;
//     Jodah, the Unifier's line);
//   - Tf2CastFromHand on a TfSelf ETB: the source entered as the resolving
//     spell its controller cast from their hand (Soulhunter Rakshasa:
//     gorge matches ValidCard$ as the event happens, so the enters check,
//     run inside the resolution, is the only read);
//   - Tf2Convoked on a TfSelf ETB: the trigger amount is the number of
//     creatures that convoked the spell, bound as it triggers (gorge
//     Convoked$Amount; Knight-Errant of Eos).
//
// A spell copy has no provenance (copy_gen.go clears prov: it was never
// cast).

// provHand: entry e was cast from its owner's hand by player p.
func (s *State) castFromHandBy(e *stackEntry, p uint8) bool {
	return e.kind == seSpell && kop.Zone(e.prov&7) == kop.ZoneHand && e.ctrl == p && s.objs[e.obj].owner() == p
}

// resSpellOf is the resolving spell's entry when o is the permanent it is
// becoming (the enters checks run inside the resolution), else nil.
func (s *State) resSpellOf(o Obj) *stackEntry {
	if s.res.active && s.res.e.kind == seSpell && s.res.e.obj == o {
		return &s.res.e
	}
	return nil
}

// bindTrigProv binds the pending trigger just queued: its X (Tf2XBound,
// or the entering spell's spend for Tf2SpentBound, trig_xpaid.go)
// and its convoke count (Tf2Convoked, into the amount).
//
//go:noinline
func (s *State) bindTrigProv(a *kop.Ability, tobj Obj) {
	if a.Trig.Flags2&kop.Tf2XBound != 0 && a.Trig.Event == kop.TrigTurnFaceUp {
		// CR 107.3m: the X announced for the turn-up rides the event's amount
		if s.npend > 0 && s.pend[s.npend-1].trigObj == tobj && tobj != NoObj {
			s.pend[s.npend-1].x = objWord(max(0, min(255, int(s.pend[s.npend-1].trigAmt))))
		}
	} else if a.Trig.Flags2&(kop.Tf2XBound|kop.Tf2SpentBound) != 0 {
		s.bindTrigX(tobj, a.Trig.Flags2&kop.Tf2SpentBound != 0)
	}
	if a.Trig.Flags2&kop.Tf2Convoked != 0 && s.npend > 0 && s.pend[s.npend-1].trigObj == tobj && tobj != NoObj {
		n := int16(0)
		if e := s.resSpellOf(tobj); e != nil {
			n = int16(e.prov >> 3)
		}
		s.pend[s.npend-1].trigAmt = n
	}
}
