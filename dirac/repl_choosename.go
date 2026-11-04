package dirac

import (
	"sync/atomic"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// The as-enters nonland card-name choice (CR 614.12, 201.4): an enters
// program opening with OpExt ExtChooseName (kgen lower_choosename.go; Kai
// Winn, Faithless Leader's NameCard replacement, Defined$ You, ValidCards$
// Card.nonLand). gorge asks it at the entry boundary as a KChoose of "name"
// options over the sorted name list of the nonland cards (effects
// NameChoicesFromListCtx), the answer kept on Object.ChosenName; the kernel
// asks it as the colour choice is (akName: preAskFor on the stack,
// queueAskAt mid-resolution) over the matchup's real nonland cards
// (Table.NameCards) and keeps the answer in the permanent's chosen byte as
// that card's CardID, so a permanent that chose nothing holds 255 and a
// permanent that left holds nothing (CR 400.7: the next entry asks anew).
//
// A SpellCast line carrying Tf4NamedCast reads the choice twice: castNamedOK
// (a cast of a spell with the chosen name, from castTrigGen) and
// namedActivation (an activation of an ability of a source with that name,
// from fireAbilityCast). Names are CardIDs: a table card has one name, and a
// multi-face card beside a name chooser is refused when the matchup is built
// (cardset pooldata_names.go; the pool table plays the chooser inert when it
// cannot fit). The activation half compares the source's printed CardID, so a
// source whose name changed (a copy of a named card, a face-down permanent) is
// told apart from the card it copies only by gorge, which reads the current
// name: no such card is in the pools.

// NamedCastStats counts what the Tf4NamedCast lines queued; oracle tests only.
type NamedCastStats struct {
	SpellFires atomic.Int64 // queued by a spell cast with the chosen name
	ActFires   atomic.Int64 // queued by an activation of a source with the chosen name
}

// DebugNamedCastStats is the test hook; nil in normal play.
var DebugNamedCastStats *NamedCastStats

// The lockstep mutation switches (TestKaiWinnMutation): DebugNamedCastNoAct
// drops the activation half; DebugNamedCastAnySpell lets every opponent spell
// match instead of the one with the chosen name.
var (
	DebugNamedCastNoAct    bool
	DebugNamedCastAnySpell bool
)

// nameOptions fills the as-enters name ask: one option per real nonland card
// of the matchup, mode = its CardID, ascending (gorge lists the names
// sorted, and the table's cards are sorted by name).
//
//go:noinline
func (s *State) nameOptions(k *prep, d *Decision) {
	d.Sub = ChGenName
	for _, c := range k.t.NameCards {
		d.add(MakeAction(AkChoose, 0, 0, uint8(c)))
	}
}

// chosenNameOf is the CardID the permanent at sl named as it entered, false
// when it named none (its chosen byte is 255) or sl holds no permanent.
func (s *State) chosenNameOf(sl Slot) (CardID, bool) {
	if sl == NoSlot || !s.occ.has(sl) || s.perms[sl].chosen == 255 {
		return 0, false
	}
	return CardID(s.perms[sl].chosen), true
}

// castNamedOK is the spell half of a Tf4NamedCast line: the spell e is cast
// by an opponent of the listener's controller (the caller's whoseOK) and
// carries the listener's chosen name. A face-down spell has no name (CR
// 708.2a).
func (s *State) castNamedOK(l trigListener, e *stackEntry) bool {
	if DebugNamedCastAnySpell {
		return true
	}
	if e.faceDown() {
		return false
	}
	c, ok := s.chosenNameOf(l.sl)
	return ok && c == e.card
}

// namedActivation is the activation half of a Tf4NamedCast line: the ability
// entry e was just activated by an opponent of the listener's controller and
// its source is a card with the listener's chosen name. A mana ability never
// reaches fireAbilityCast (it does not use the stack; gorge's AbilityCast
// fires on a stack activation only).
func (s *State) namedActivation(k *prep, e *stackEntry, l trigListener, a *kop.Ability) bool {
	if DebugNamedCastNoAct || !whoseOK(a.Trig.Whose, l.ctrl, e.ctrl) {
		return false
	}
	c, ok := s.chosenNameOf(l.sl)
	return ok && c == e.card
}

// noteNamedCast counts one queued Tf4NamedCast trigger (oracle tests).
func noteNamedCast(spell bool) {
	st := DebugNamedCastStats
	if st == nil {
		return
	}
	if spell {
		st.SpellFires.Add(1)
	} else {
		st.ActFires.Add(1)
	}
}
