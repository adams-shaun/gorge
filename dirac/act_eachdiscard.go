package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// One discard effect's players choose in turn and then everything they chose
// moves at once (CR 101.4; gorge effDiscard, flushDiscards), so a later player
// chooses with the earlier cards still in hand. The held discard keeps every
// earlier player's choice, one block each in answer order; the last player's
// answer flushes them all, first player first, then its own.

// A held block is a header byte and maxDiscHeld pick bytes. discHeldWhole in
// the header holds a player's whole hand, its seat in the first pick byte; any
// other nonzero header is how many picks follow, in answer order; zero is an
// empty block. maxDiscHeld is one per hand card: a discard picks at most the
// whole hand, and the held picks must keep the answer order (CR 404.3). There
// are MaxSeats-1 blocks: the last player's choice is never held.
const (
	discHeldWhole = 255
	maxDiscHeld   = MaxHand
	discHeldBlock = 1 + maxDiscHeld
	discHeldLen   = (MaxSeats - 1) * discHeldBlock
)

// startEachDiscard poses OpDiscard's "each player discards N" player by
// player, controller first. A player whose whole hand goes poses no ask and the
// chain moves on; a live ask leaves the players still to come in g.repNext for
// applyResolve's raDiscard resume.
func (s *State) startEachDiscard(k *prep, in *kop.Instr, p uint8) bool {
	q := s.eachPlayers(k, in.Obj)
	if p != q.at(0) {
		q = seatOne(p)
	}
	return s.discardSeq(k, in, q)
}

// discardSeq asks each player of q in order for its discard.
func (s *State) discardSeq(k *prep, in *kop.Instr, q seatSeq) bool {
	s.repRest(q, 1)
	return s.discardChain(k, in, q.at(0))
}

// discardChain asks p for its discard and then each player the chain holds
// (g.repNext). It reports whether the chain finished, the cards moved, without
// a live ask.
func (s *State) discardChain(k *prep, in *kop.Instr, p uint8) bool {
	for p != 255 {
		// gorge discardBounds keeps a resolved zero at zero (Death Cloud's X
		// of 0): such a player discards nothing and is not asked
		if n := s.value(k, in.A); n >= 1 {
			pl := &s.pl[p]
			if n < int(pl.handN) {
				s.res.ask = raDiscard
				s.res.peekI = 0
				s.res.askP = p
				s.res.left = uint8(n)
				s.res.chosen = p
				return false
			}
			if s.repPending() {
				s.holdWhole(p)
			} else {
				s.flushHeldDiscard(k)
				for pl.handN > 0 {
					s.discardObj(k, pl.hand[0])
				}
			}
		} else if !s.repPending() {
			s.flushHeldDiscard(k)
		}
		p = s.repPop()
	}
	return true
}

// tableHasOp: some ability of t runs op.
func tableHasOp(t *kop.Table, op kop.Op) bool {
	for i := range t.Cards {
		for j := range t.Cards[i].Abilities {
			for _, in := range t.Cards[i].Abilities[j].Prog {
				if in.Op == op {
					return true
				}
			}
		}
	}
	return false
}

// heldBlock is the first empty held block.
func (s *State) heldBlock() []uint8 {
	h := s.cold.discHeld
	for i := 0; i+discHeldBlock <= len(h); i += discHeldBlock {
		if h[i] == 0 {
			return h[i : i+discHeldBlock]
		}
	}
	return nil // unreachable: a chain holds at most MaxSeats-1 players
}

// holdDiscard keeps the answering player's picks until the last player has chosen.
func (s *State) holdDiscard(picks []Obj) {
	b := s.heldBlock()
	b[0] = uint8(len(picks))
	for i, o := range picks {
		b[1+i] = uint8(o)
	}
}

// holdWhole keeps player p's whole hand until the last player has chosen.
func (s *State) holdWhole(p uint8) {
	b := s.heldBlock()
	b[0] = discHeldWhole
	b[1] = p
}

// flushHeldDiscard moves every held discard, first player first.
func (s *State) flushHeldDiscard(k *prep) {
	h := s.cold.discHeld
	for i := 0; i+discHeldBlock <= len(h) && h[i] != 0; i += discHeldBlock {
		n := h[i]
		h[i] = 0
		if n == discHeldWhole {
			for pl := &s.pl[h[i+1]]; pl.handN > 0; {
				s.discardObj(k, pl.hand[0])
			}
			continue
		}
		var picks [maxDiscHeld]Obj
		for j, o := range h[i+1 : i+1+int(n)] {
			picks[j] = Obj(o)
		}
		s.discardPicks(k, picks[:n], 0)
	}
}

// discardChained: the discard instruction in resumes a chain after each answer
// (an each-player op, or several targeted players; a single-player ask must not
// pick up a stale repNext).
func (s *State) discardChained(k *prep, in *kop.Instr) bool {
	if in.Obj == kop.RefEachPlayer {
		return true
	}
	sl, ok := tgtSlot(in.Obj)
	return ok && k.gen != nil && s.res.e.xi != 0 && s.slotHasExtra(&s.res.e, uint8(sl))
}
