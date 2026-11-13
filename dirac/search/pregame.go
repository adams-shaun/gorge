package search

import (
	"slices"

	"github.com/adams-shaun/gorge/dirac"
)

// Pre-game helpers for a host's value-based opening decisions (:mullvalue,
// :playdraw): states the caller values with the seat's Evaluator outside
// any search. They live in their own file, out of line, so the hot files
// keep their layout.

// NewEpoch returns a fresh epoch, unique across every Searcher of the
// process (the counter Search draws from), for a state the caller
// evaluates outside a search, so no evaluator's base-slot cache mistakes it
// for a search's root.
func NewEpoch() uint32 {
	e := epochs.Add(1)
	if e == 0 {
		e = epochs.Add(1)
	}
	return e
}

// DealUnseen deals every hidden card of s, every seat's hand and library,
// for a chooser (seat) that has seen neither hand: the coin toss's winner
// choosing to play or draw. Seat's own hand and library are dealt from its
// own cards; the opponent's from the closed-list corpus under ClosedList
// with a Belief, else from its own cards. Each pool is sorted before the
// deal, so the result is a function of the decklists and (a, b) alone and
// never of which cards either real hand holds (Clairvoyant is ignored). The
// state's chance is re-seeded with b, as a search world's is. The learned
// beliefs (beliefnet.go) are not used here: they deal a Search's worlds only.
func (sr *Searcher) DealUnseen(s *dirac.State, seat uint8, a, b uint64) {
	r := dirac.NewRand(a, b)
	for p := uint8(0); p < s.NumSeats(); p++ {
		n := s.HiddenCards(p, sr.pool[:])
		pool := sr.pool[:n]
		if p != seat && sr.Opts.ClosedList && sr.Belief != nil {
			ns := s.SeenCards(p, seat, sr.seenC[:])
			seen := sr.seenC[:ns]
			slices.Sort(seen)
			var mask uint8
			for _, c := range seen {
				mask |= CardColours(s.Table(), c)
			}
			sr.Belief.deal(&r, seen, mask, n, pool)
		} else {
			slices.Sort(pool)
		}
		s.DealHidden(p, &r, pool)
	}
	s.ReseedChance(b, b^0x9e3779b97f4a7c15)
}
