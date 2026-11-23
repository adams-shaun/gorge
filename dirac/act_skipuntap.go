package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// useSkipUntapMark: some card's program is an OpSkipUntap. Its per-permanent
// marks (one bit per seat) live in a per-slot cold region, so a table without
// one pays nothing and perm.cseen keeps no seat-sized field.
var useSkipUntapMark = newGenUse(func(t *kop.Table) bool {
	for i := range t.Cards {
		if skipUntapCard(&t.Cards[i]) {
			return true
		}
	}
	return false
})

// coldSkipUntap is each battlefield slot's OpSkipUntap marks: bit p is set
// when seat p's effect keeps the permanent tapped through p's next untap step.
// A perSlot region, zeroed when the slot takes a new permanent (CR 400.7: the
// mark leaves with it).
var coldSkipUntap = newCold[uint8]("skipUntap", func(g *genPrep) int {
	if g != nil && g.uses.has(useSkipUntapMark) {
		return MaxPerm
	}
	return 0
}).perSlot()

// skipUntapMark is sl's OpSkipUntap mark bits (bit p: seat p's effect).
func (s *State) skipUntapMark(sl Slot) uint8 {
	if c := coldSkipUntap.of(s); int(sl) < len(c) {
		return c[sl]
	}
	return 0
}

// pfSkipUntap is CR 611.2b's "doesn't untap during your next untap step"
// one-shot (gorge Object.CantUntapNextStep): the untap step skips the
// permanent and clears it (turn.go, act_energy.go untapStepGen); a new
// permanent starts without it. Bit 15 of perm.flags.
const pfSkipUntap uint16 = 1 << 15

// noUntapCreat shifts a seat's bit in gen.noUntapL from its lands to its
// creatures (SkipUntapPlayerCL): bits 0-3 the lands, bits 4-7 the creatures.
const noUntapCreat = 1 << 4

// prepSkipUntap sets genPrep.anySkipUntap when a program makes a
// "doesn't untap during your next untap step" effect (OpSkipUntap).
func prepSkipUntap(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			a := &t.Cards[i].Abilities[j]
			for _, in := range a.Prog {
				g.anySkipUntap = g.anySkipUntap || in.Op == kop.OpSkipUntap
				if in.Op == kop.OpSkipUntap && in.C >= kop.SkipUntapLockStep {
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
	if in.C == kop.SkipUntapLockStep || in.C == kop.SkipUntapLockAny || in.C == kop.SkipUntapLockHostStep || in.C == kop.SkipUntapLockUntapStep {
		s.untapLockAdd(k, in) // act_untaplock.go
		return
	}
	if in.C == kop.SkipUntapPlayerCL {
		if q := s.playerRef(k, in.Obj); q != 255 {
			s.g.noUntapL |= 1<<q | noUntapCreat<<q
		}
		return
	}
	if in.C == 1 {
		s.g.noUntapL |= 1 << p
		return
	}
	if sl := s.objRef(k, in.Obj); sl != NoSlot {
		s.skipUntapSelf(sl, p)
	}
}

// skipUntapSelf marks the permanent at sl for player p's next untap step
// (OpSkipUntap C=0). A mana ability's stay-tapped rider calls it directly
// (mana_rider.go) with the source's controller: the activation is no
// resolving program, so there is no resolution context to resolve RefSelf
// through.
func (s *State) skipUntapSelf(sl Slot, p uint8) {
	if c := coldSkipUntap.of(s); int(sl) < len(c) {
		c[sl] |= 1 << SeatIx(p)
		s.touchSl(sl)
	}
}

// skipsUntap: the active player's untap step does not untap sl (an
// OpSkipUntap effect of the active player on it, or on their lands). The
// source-bound untap locks are no part of it: gorge's Effect replacement meets
// the untap after the keep-tapped election, so untapStepGen reads
// untapLocked itself, once the election is answered.
func (s *State) skipsUntap(k *prep, sl Slot) bool {
	a := s.active
	if s.skipUntapMark(sl)&(1<<SeatIx(a)) != 0 {
		return true
	}
	if s.g.noUntapL == 0 || s.perms[sl].ctrl != a {
		return false
	}
	ty := s.typesGen(k, sl)
	return s.g.noUntapL&(1<<a) != 0 && ty&kop.TLand != 0 || s.g.noUntapL&(noUntapCreat<<a) != 0 && ty&kop.TCreature != 0
}

// expireSkipUntap ends the active player's OpSkipUntap effects as their
// untap step ends (gorge expireUntapStepEffects: every such effect the
// active player controls, applied or not).
//
//go:noinline
func (s *State) expireSkipUntap() {
	a := s.active
	s.g.noUntapL &^= 1<<a | noUntapCreat<<a
	c := coldSkipUntap.of(s)
	for m := s.occ; m.any() && len(c) != 0; m = m.rest() {
		sl := m.first()
		if c[sl]&(1<<SeatIx(a)) != 0 {
			c[sl] &^= 1 << SeatIx(a)
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
