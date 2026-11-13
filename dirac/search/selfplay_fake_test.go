package search

import (
	"github.com/adams-shaun/gorge/dirac"
	v "github.com/adams-shaun/gorge/dirac/vanilla"
)

// Test helpers for the self-play options (seat.go, forced.go): a fake
// network and a seat-search game.

// fakeNet is a network stand-in: a real prior (PriorSource) whose logit
// for option i depends on i and the option, a constant leaf value (or the
// heuristic leaf with val < 0), and a count of its Priors calls.
type fakeNet struct {
	val    float32
	flat   bool // every logit 0: the uniform prior through the network path
	priors int
}

func (f *fakeNet) HasPrior() bool { return true }

func (f *fakeNet) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	if f.val < 0 {
		return HeuristicLeaf{}.Value(s, d, seat)
	}
	return f.val
}

func (f *fakeNet) Priors(_ *dirac.State, d *dirac.Decision, _ []bool, out []float32) {
	f.priors++
	for i := 0; i < int(d.N) && i < len(out); i++ {
		if f.flat {
			out[i] = 0
			continue
		}
		out[i] = float32((uint32(twoSeatCodes(d.Options[i]))*2654435761>>7)%13) / 4
	}
}

// twoSeatCodes renumbers an action's source and target bytes as the two-seat
// build numbers them, so the fake prior, and every digest pinned over it, is
// the same in both size classes: the four-seat class starts the stack and
// player codes two lower (StackTarget0 0xEA, PlayerTarget0 0xFA). In the
// two-seat build it returns a unchanged.
func twoSeatCodes(a dirac.Action) dirac.Action {
	const shift = 0xEC - dirac.StackTarget0
	if src := a.Source(); src >= dirac.StackTarget0 && src < 0xFE {
		a = a&^0xFF0000 | dirac.Action(src+shift)<<16
	}
	if t := a.Target(); t >= dirac.StackTarget0 && t < 0xFE {
		a = a&^0xFF00 | dirac.Action(t+shift)<<8
	}
	return a
}

// seatStep is one decision of a seat-search game: the searching seat's
// result (a copy) or, for the other seat, nothing.
type seatStep struct {
	res Result
}

// seatGame plays one vanilla game from seed: seat 0 answers with
// sr.SeatSearch (one SeatState for the game), seat 1 uniformly at random.
// It returns seat 0's results in order and the winner.
func seatGame(sr *Searcher, seed uint64) ([]seatStep, int8) {
	var st SeatState
	return seatGameState(sr, seed, &st)
}

// seatGameState is seatGame with the caller's SeatState.
func seatGameState(sr *Searcher, seed uint64, st *SeatState) ([]seatStep, int8) {
	var s dirac.State
	s.Init(v.Table, v.Decks[seed%5], v.Decks[(seed+2)%5], seed, dirac.Config{})
	pol := dirac.NewRandomPolicy(seed ^ 0x1234)
	var d dirac.Decision
	var out []seatStep
	for i := 0; i < 3000; i++ {
		s.StepInto(&d)
		if d.Kind == dirac.DkNone {
			break
		}
		if d.Player == 0 {
			r := sr.SeatSearch(&s, &d, seed*0x51, st)
			out = append(out, seatStep{res: *r})
			r.Play(&s)
			continue
		}
		s.Apply(pol.Pick(&d))
	}
	_, w := s.Terminal()
	return out, w
}

// sameSearch reports whether two results searched the same tree to the same
// answer (everything but the SeatSearch fields).
func sameSearch(a, b *Result) bool {
	return a.N == b.N && a.Keys == b.Keys && a.Visits == b.Visits && a.Q == b.Q && a.Avail == b.Avail &&
		a.Prior == b.Prior && a.RootValue == b.RootValue && a.Choice == b.Choice && a.NActs == b.NActs &&
		a.Acts == b.Acts && a.Target == b.Target && a.HasTarget == b.HasTarget
}
