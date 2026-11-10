package dirac

import (
	"github.com/adams-shaun/gorge/dirac/kop"
)

// One discard effect's players choose in turn and then everything they chose
// moves at once (CR 101.4; gorge effDiscard, flushDiscards), so a later player
// chooses with the earlier cards still in hand. The chain holds at most the first
// player's discard: there are two seats and the last player's flushes it.

// discHeldWhole, in cold.discHeld[0], holds the first player's whole hand;
// any other value is how many of cold.discHeld[1:] are held, in answer order.
// maxDiscHeld is one per hand card: a discard picks at most the whole hand, and
// the held picks must keep the answer order (CR 404.3).
const (
	discHeldWhole = 255
	maxDiscHeld   = MaxHand
)

// startEachDiscard poses OpDiscard's "each player discards N" player by
// player, controller first. A player whose whole hand goes poses no ask and the
// chain moves on; a live ask records the player still to come in g.repNext for
// applyResolve's raDiscard resume.
func (s *State) startEachDiscard(k *prep, in *kop.Instr, p uint8) bool {
	// two seats only: the held discard (coldArrays.discHeld) keeps one
	// earlier player's picks, so a third player's ask has no room yet
	q := s.eachPlayers(k, in.Obj)
	next := uint8(255)
	if p == q.at(0) {
		next = q.at(1)
	}
	return s.discardChain(k, in, p, next)
}

// discardChain asks p for its discard and then next (255: none). It reports
// whether the chain finished, the cards moved, without a live ask.
func (s *State) discardChain(k *prep, in *kop.Instr, p, next uint8) bool {
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
				s.g.repNext = next
				return false
			}
			if next != 255 {
				s.cold.discHeld[0] = discHeldWhole
			} else {
				s.flushHeldDiscard(k, p^1)
				for pl.handN > 0 {
					s.discardObj(k, pl.hand[0])
				}
			}
		} else if next == 255 {
			s.flushHeldDiscard(k, p^1)
		}
		p, next = next, 255
		s.g.repNext = 255
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

// holdDiscard keeps the answering player's picks until the last player has chosen.
func (s *State) holdDiscard(picks []Obj) {
	s.cold.discHeld[0] = uint8(len(picks))
	for i, o := range picks {
		s.cold.discHeld[1+i] = uint8(o)
	}
}

// flushHeldDiscard moves the first player's held discard, if any.
func (s *State) flushHeldDiscard(k *prep, first uint8) {
	held := s.cold.discHeld
	if len(held) == 0 || held[0] == 0 {
		return
	}
	n := held[0]
	held[0] = 0
	if n == discHeldWhole {
		for pl := &s.pl[first]; pl.handN > 0; {
			s.discardObj(k, pl.hand[0])
		}
		return
	}
	var picks [maxDiscHeld]Obj
	for i, o := range held[1 : 1+int(n)] {
		picks[i] = Obj(o)
	}
	s.discardPicks(k, picks[:n], 0)
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
