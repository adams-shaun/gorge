package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// pfSkipUntap is CR 611.2b's "doesn't untap during your next untap step"
// one-shot (gorge Object.CantUntapNextStep): the untap step skips the
// permanent and clears it (turn.go, act_energy.go untapStepGen); a new
// permanent starts without it. Bit 15 of perm.flags (pfRegenX holds 13-14).
const pfSkipUntap uint16 = 1 << 15

// prepSkipUntap sets genPrep.anySkipUntap when a program makes a
// "doesn't untap during your next untap step" effect (OpSkipUntap).
func prepSkipUntap(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			for _, in := range a.Prog {
				g.anySkipUntap = g.anySkipUntap || in.Op == kop.OpSkipUntap
				if in.Op == kop.OpSkipUntap && in.C >= kop.SkipUntapLockStep {
					g.anyUntapLock = true
					g.anyUntapBlock = g.anyUntapBlock || in.C == kop.SkipUntapLockAny // untapBlocked reads the any-event locks
				}
			}
		}
	}
}

// skipUntap is OpSkipUntap (gorge's Effect with an Untap replacement,
// ValidStepTurnToController$ Player.Activator, Duration$
// UntilTheEndOfYourNextUntap): C=0 marks the permanent Obj for the
// controller's next untap step (the mark leaves with it: ExileOnMoved$
// Battlefield); C=1 the controller's lands (Land.YouCtrl, read then). One mark
// holds any number of them: gorge asks no order at the untap (untapDenied), and all
// of a player's end with that player's next untap step.
func (s *State) skipUntap(k *prep, in *kop.Instr) {
	p := s.res.e.ctrl
	if in.C >= kop.SkipUntapLockStep {
		s.untapLockAdd(k, in) // act_untaplock.go
		return
	}
	if in.C == 1 {
		s.g.noUntapL |= 1 << p
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		s.perms[sl].cseen |= cseenSkipUntap0 << p
		s.touchSl(sl)
	}
}

// skipsUntap: the active player's untap step does not untap sl (an
// OpSkipUntap effect of the active player on it, or on their lands).
func (s *State) skipsUntap(k *prep, sl Slot) bool {
	a := s.active
	return s.perms[sl].cseen&(cseenSkipUntap0<<a) != 0 || s.g.nul != 0 && s.untapLocked(s.perms[sl].obj, false) ||
		s.g.noUntapL&(1<<a) != 0 && s.perms[sl].ctrl == a && s.typesGen(k, sl)&kop.TLand != 0
}

// expireSkipUntap ends the active player's OpSkipUntap effects as their
// untap step ends (gorge expireUntapStepEffects: every such effect the
// active player controls, applied or not).
//
//go:noinline
func (s *State) expireSkipUntap() {
	a := s.active
	s.g.noUntapL &^= 1 << a
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if s.perms[sl].cseen&(cseenSkipUntap0<<a) != 0 {
			s.perms[sl].cseen &^= cseenSkipUntap0 << a
			s.touchSl(sl)
		}
	}
}

// skipNextUntapAll is OpSkipNextUntap's sweep form (Sleep):
// every battlefield permanent matching in.F -- only those the player in.Obj
// controls when it names one -- doesn't untap during its controller's next
// untap step (gorge registerPumpEffects stamps CantUntapNextStep on each
// swept object; cr-sleep-defined-targeted scopes Defined$ Targeted to the
// parent's player target).
//
//go:noinline
func (s *State) skipNextUntapAll(k *prep, in *kop.Instr) {
	m := s.occ
	if in.Obj != kop.RefNone {
		p := s.playerRef(k, in.Obj)
		if p == 255 {
			return
		}
		m = m.and(s.ctrlM[p])
	}
	src := s.objRef(k, kop.RefSelf)
	ctrl := s.res.e.ctrl
	for ; m.any(); m = m.rest() {
		sl := m.first()
		if s.matchPerm(k, in.F, sl, ctrl, src) {
			s.touchSl(sl)
			s.perms[sl].flags |= pfSkipUntap
		}
	}
}
