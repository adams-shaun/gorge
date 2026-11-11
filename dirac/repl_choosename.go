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
func (s *State) nameOptions(k *prep, d *Decision, kind uint8) {
	names := k.t.NameCards
	if kind != akName {
		names = k.t.NameCardsAny
	}
	if kind == akNameHand {
		d.Sub = ChGenName
		m := s.revealedNames(k, SeatIx(d.Player))
		for c := 0; c < 256; c++ {
			if m[c>>6]>>(c&63)&1 != 0 {
				d.add(MakeAction(AkChoose, 0, 0, uint8(c)))
			}
		}
		return
	}
	if kind == akNameLook {
		if s.pl[s.nextAlive(SeatIx(d.Player))].handN > 0 { // the one opponent: Spyglass's ChoosePlayer is two-seat only (lower_chooseplayer.go)
			d.Sub = ChGenLookAck // gorge's one-option look_ack before the look (poseLookAck)
			d.add(MakeAction(AkChoose, 0, 0, 0))
			return
		}
		s.nameLookDone()
	}
	d.Sub = ChGenName
	for _, c := range names {
		d.add(MakeAction(AkChoose, 0, 0, uint8(c)))
	}
}

// revealedNames is the set of CardIDs of the nonland cards in the hands of
// every seat still in the game but p's: what Alhammarret, High Arbiter's
// reveal shows p (gorge lists the names sorted; the CardIDs ascend with the
// names, and a nonland CardID is under 255, cardset nameChoices).
func (s *State) revealedNames(k *prep, p uint8) (m [4]uint64) {
	for q := uint8(0); q < s.nSeats(); q++ {
		if q == p || s.out(q) {
			continue
		}
		pl := &s.pl[q]
		for i := 0; i < int(pl.handN); i++ {
			if c := s.objs[pl.hand[i]].card; !k.cards[c].land && c < 255 {
				m[c>>6] |= 1 << (c & 63)
			}
		}
	}
	return m
}

// nameHandNone reports that the opponents of p revealed no nonland card, so
// the choice of Alhammarret's name is not asked (gorge: an empty eligible set
// poses no decision and chooses nothing).
func (s *State) nameHandNone(k *prep, p uint8) bool {
	m := s.revealedNames(k, p)
	return m == [4]uint64{}
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

// nameAskKind is the ask kind of an ExtChooseName instruction: B = 0 the
// nonland names (Kai Winn), 1 every card's name, 2 every card's name after a
// look at the opponent's hand (Sorcerous Spyglass: ChoosePlayer, RevealHand
// Look$ True, NameCard; the lookless hand is posed no look_ack), 3 the nonland
// names among the opponents' hands, revealed as it enters (Alhammarret, High
// Arbiter: RevealHand, NameCard ChooseFromDefinedCards$ Remembered; no ask
// when none was shown).
func nameAskKind(in *kop.Instr) uint8 {
	switch in.B.N {
	case 1:
		return akNameAny
	case 2:
		return akNameLook
	case 3:
		return akNameHand
	}
	return akName
}

// isNameAsk reports an as-enters card-name ask kind.
func isNameAsk(kind uint8) bool {
	return kind == akName || kind == akNameAny || kind == akNameLook || kind == akNameHand
}

// nameLookDone turns a pending look-then-name ask into the plain name ask
// (the opponent's hand is empty, so gorge poses no look_ack).
func (s *State) nameLookDone() {
	if s.g.pre.state == 1 && s.g.pre.kind == akNameLook {
		s.g.pre.kind = akNameAny
	}
	if s.g.nask > 0 && s.g.ask[0].kind == akNameLook {
		s.g.ask[0].kind = akNameAny
	}
}
