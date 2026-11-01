package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// ChoiceRestriction$ on a charm ("choose one that hasn't been chosen this
// turn / this game"). gorge keeps a per-source pick log
// (state.Object.ModeChoices): a pick is recorded when the mode is announced
// (effects.RecordCharmChoices at a trigger's placement, an activation's
// announcement), a later offer of the same charm on the same source drops the
// modes picked under its scope (effects.CharmEligibleModes), the turn change
// drops the ThisTurn picks, and leaving the battlefield drops them all (CR
// 400.7).
//
// The kernel keeps the log in State.g.chPick, one entry per source slot: the
// picked mode indices (bits 0-3) and the scope the source's one restricted
// charm uses (bit 7 = ThisGame; kgen refuses a second restricted charm on a
// card, so one scope per slot). The leave reset runs at the slot's next enter
// (enterGen): no reader runs between a leave and the next enter, because a
// reader is the source's own trigger or activation, and both need the
// permanent on the battlefield (a trigger whose source left finds no slot,
// srcSlot, and offers every mode as gorge's emptied log does).
//
// Everything is behind genPrep.anyCharmPick (recording, offering) or
// genState.nchPick != 0 (the resets).

// DebugCharmPickNoTurnReset keeps the ThisTurn picks across the turn change
// (mutation test only).
var DebugCharmPickNoTurnReset bool

// DebugCharmPickNoGameScope records a ThisGame pick as a ThisTurn one, so it
// clears at the turn change (mutation test only).
var DebugCharmPickNoGameScope bool

const cpGame = 0x80 // chPick m bit: the ThisGame scope

func init() { stallNames[StallCharmPick] = "charmpick" }

// prepCharmPick sets anyCharmPick: some ability carries ChoiceRestriction$.
func prepCharmPick(t *kop.Table, g *genPrep) {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			g.anyCharmPick = g.anyCharmPick || t.Cards[i].Abilities[j].ModeRestrict != 0
		}
	}
}

// charmPickAllBarred reports whether every mode of a was picked before.
func (s *State) charmPickAllBarred(a *kop.Ability, src Slot) bool {
	for m := 0; m < int(a.NModes); m++ {
		if !s.charmPickBarred(a, src, m) {
			return false
		}
	}
	return a.ModeRestrict != 0
}

// charmPickAt is the index of slot sl's entry, or -1.
func (s *State) charmPickAt(sl Slot) int {
	for i := 0; i < int(s.g.nchPick); i++ {
		if s.g.chPick[i].sl == sl {
			return i
		}
	}
	return -1
}

// charmPickBarred reports whether mode m of a, a restricted charm on the
// permanent at src, was picked before under a's scope.
//
//go:noinline
func (s *State) charmPickBarred(a *kop.Ability, src Slot, m int) bool {
	if a.ModeRestrict == 0 || src == NoSlot {
		return false
	}
	i := s.charmPickAt(src)
	if i < 0 {
		return false
	}
	e := s.g.chPick[i].m
	return (e&cpGame != 0) == (a.ModeRestrict == 2) && e&(1<<m) != 0
}

// charmPickRecord logs mode m, just announced for the entry being built, on
// its source (gorge RecordCharmChoices; a source that left the battlefield
// has no log to read, so nothing is kept).
//
//go:noinline
func (s *State) charmPickRecord(k *prep, m uint8) {
	if a := s.buildAbility(k); a != nil {
		s.charmPickLog(a, s.srcSlot(), m)
	}
}

// charmPickLog logs mode m of a, a restricted charm on the permanent at sl.
//
//go:noinline
func (s *State) charmPickLog(a *kop.Ability, sl Slot, m uint8) {
	if a.ModeRestrict == 0 || sl == NoSlot {
		return
	}
	i := s.charmPickAt(sl)
	if i < 0 {
		if int(s.g.nchPick) == len(s.g.chPick) {
			s.charmPickSweep()
		}
		if int(s.g.nchPick) == len(s.g.chPick) {
			s.capFull(StallCharmPick)
			return
		}
		i = int(s.g.nchPick)
		s.g.nchPick++
		s.g.chPick[i].sl = sl
		s.g.chPick[i].m = 0
		if a.ModeRestrict == 2 && !DebugCharmPickNoGameScope {
			s.g.chPick[i].m = cpGame
		}
	}
	s.g.chPick[i].m |= 1 << m
}

// charmPickDel drops entry i, keeping the order of the rest.
func (s *State) charmPickDel(i int) {
	n := int(s.g.nchPick) - 1
	copy(s.g.chPick[i:n], s.g.chPick[i+1:n+1])
	s.g.chPick[n] = struct {
		sl Slot
		m  uint8
	}{}
	s.g.nchPick = uint8(n)
}

// charmPickSweep drops the entries of slots no permanent occupies (their
// source left; the entry would be dropped at the slot's next enter anyway).
func (s *State) charmPickSweep() {
	for i := int(s.g.nchPick) - 1; i >= 0; i-- {
		if !s.occ.has(s.g.chPick[i].sl) {
			s.charmPickDel(i)
		}
	}
}

// charmPickLeft drops slot sl's entry: a permanent enters it (the last one's
// log ended when it left, CR 400.7).
//
//go:noinline
func (s *State) charmPickLeft(sl Slot) {
	if i := s.charmPickAt(sl); i >= 0 {
		s.charmPickDel(i)
	}
}

// charmPickTurn drops the ThisTurn picks at the turn change.
//
//go:noinline
func (s *State) charmPickTurn() {
	if DebugCharmPickNoTurnReset {
		return
	}
	for i := int(s.g.nchPick) - 1; i >= 0; i-- {
		if s.g.chPick[i].m&cpGame == 0 {
			s.charmPickDel(i)
		}
	}
}
