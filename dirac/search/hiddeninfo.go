package search

import (
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// Hidden-information and value keys, written from a design note's
// description (every one is 0 = off, the search exactly as without it):
//
//   - HoldBonus (:holdbonus=LAMBDA): at a non-terminal
//     leaf on the opponent's turn the seat's value gains LAMBDA when the
//     seat holds an instant-speed card (an instant, or flash; not a land)
//     whose mana value its untapped lands cover. Speed and mana value only:
//     colours, other mana sources and the card's text are not read.
//   - OppView (:oppview=RHO): a multiple-
//     observer approximation. In a fraction RHO of simulations, at the first
//     opponent point the walk reaches, the seat's hidden cards (hand and
//     library: what the opponent cannot see) are dealt again from their own
//     pool, and that simulation's value is backed up through the opponent's
//     points only, so the tree's opponent answers believable hands. Every
//     other simulation (and one that never reaches an opponent point) backs
//     up as before: the seat's statistics, the root's included, come from
//     real-hand simulations.
//   - OppFair (:oppfair=F): an MO-ISMCTS
//     approximation over the same re-deal: a fraction F of simulations
//     re-deal the seat's hand at the first opponent point and update only
//     the opponent's points; the real-hand simulations update only the
//     seat's points (the root's included).
//
// A node whose first visit a simulation may not update (a seat point
// expanded by a re-dealt simulation; an opponent point expanded by an
// OppFair real-hand one) is left unvisited (n 0, Q read as 0.5) for the
// next simulation that may. OppView and OppFair run on the serial tree only
// (Validate refuses Batch and NodeCache with them).

// redeal modes of the current simulation (Searcher.hid*)
const (
	hidAll  uint8 = iota // update every point (also: the key is off)
	hidOpp               // a re-dealt simulation: opponent points only
	hidSeat              // an OppFair real-hand simulation: seat points only
)

// validateInfo checks the hidden-information and value keys (Validate).
func (o *Options) validateInfo() error {
	switch {
	case !(o.HoldBonus >= 0) || math.IsInf(o.HoldBonus, 1):
		return fmt.Errorf("search: hold bonus %g must be finite and >= 0 (0 is off)", o.HoldBonus)
	case !(o.OppView >= 0 && o.OppView <= 1):
		return fmt.Errorf("search: oppview %g must be in [0,1] (0 is off)", o.OppView)
	case !(o.OppFair >= 0 && o.OppFair <= 1):
		return fmt.Errorf("search: oppfair %g must be in [0,1] (0 is off)", o.OppFair)
	case o.OppView > 0 && o.OppFair > 0:
		return fmt.Errorf("search: oppview and oppfair are exclusive")
	case (o.OppView > 0 || o.OppFair > 0) && o.Batch > 0:
		return fmt.Errorf("search: oppview/oppfair run on the serial tree (batch %d)", o.Batch)
	case (o.OppView > 0 || o.OppFair > 0) && o.NodeCache > 0:
		return fmt.Errorf("search: oppview/oppfair are incompatible with the node cache (it replays one world)")
	}
	return nil
}

// infoOn reports whether a hidden-information key is on (Search sets
// Searcher.hidOn from it once per search).
func (o *Options) infoOn() bool { return o.OppView > 0 || o.OppFair > 0 }

// hidBegin picks simulation sim's mode (runTree, before its walk): a
// re-deal with probability OppView or OppFair, by a hash of the decision's
// seed and the simulation index (deterministic, no stream consumed).
func (sr *Searcher) hidBegin(sim int) {
	f := sr.Opts.OppView
	if f == 0 {
		f = sr.Opts.OppFair
	}
	u := float64(splitmix(sr.seed^0x6f707076696577ff^splitmix(uint64(sim)+7))>>11) / (1 << 53)
	sr.hidSim, sr.hidDealt, sr.hidMode = u < f, false, hidAll
	if !sr.hidSim && sr.Opts.OppFair > 0 {
		sr.hidMode = hidSeat
	}
	sr.hidN = uint64(sim)
}

// hidRedeal deals the seat's hidden cards again in the current world (at
// the walk's first opponent point of a re-deal simulation): the hand and
// library from their own pool (the library only, if the hand is revealed
// to the opponent), shuffled by a stream of the decision's seed and the
// simulation index.
func (sr *Searcher) hidRedeal(s *dirac.State) {
	sr.hidDealt, sr.hidMode = true, hidOpp
	sr.St.OppViewSims++
	a := splitmix(sr.seed ^ 0x7265646561c0ffee ^ splitmix(sr.hidN+11))
	r := dirac.NewRand(a, splitmix(a^0x9e3779b97f4a7c15))
	p := sr.seat
	if revealedToOpponent(s, p) {
		s.RedealOwnLibrary(p, &r)
		return
	}
	n := s.HiddenCards(p, sr.pool[:])
	s.DealHidden(p, &r, sr.pool[:n])
}

// revealedToOpponent reports whether seat p's hand is revealed to an opponent
// that is still in the game: the opponents' view keeps the hand an opponent
// already sees (with two seats, the one opponent).
func revealedToOpponent(s *dirac.State, p uint8) bool {
	for q := uint8(0); q < s.NumSeats(); q++ {
		if q != p && !s.FeatLost(q) && s.HandRevealed(p, q) {
			return true
		}
	}
	return false
}

// hidWants reports whether the current simulation updates a point owned by
// the opponent (opp) or by the seat.
func (sr *Searcher) hidWants(opp bool) bool {
	switch sr.hidMode {
	case hidOpp:
		return opp
	case hidSeat:
		return !opp
	}
	return true
}

// commitInfo is commit for a simulation that updates only one side's
// points: each path entry's node and edge, and the availability marks
// taken at that node, only where hidWants the owner of the decision the
// walk met there (pathOpp: a node's decision can belong to the other seat
// in another world, so the walk's point decides, as it does for the
// opponent's 1 - Q in selection).
func (sr *Searcher) commitInfo(depth, nm int, v float64) {
	t := &sr.t
	if sr.hidMode == hidOpp {
		sr.hidDealtCommits++
	}
	for i := 0; i < depth; i++ {
		if !sr.hidWants(sr.pathOpp[i]) {
			continue
		}
		nd := &t.nodes[sr.pathN[i]]
		nd.n++
		nd.w += v
		e := &t.edges[sr.pathE[i]]
		e.n++
		e.w += v
		if t.vec {
			t.addVec(sr.pathN[i], sr.pathE[i], &sr.vec)
		}
		hi := nm
		if i+1 < depth {
			hi = int(sr.markLo[i+1])
		}
		for j := int(sr.markLo[i]); j < hi; j++ {
			t.edges[sr.marks[j]].avail++
		}
	}
}

// holdBonus is HoldBonus's term for the searching seat at leaf position s.
func (sr *Searcher) holdBonus(s *dirac.State) float64 {
	seat := sr.seat
	if s.ActivePlayer() == seat || s.Over() {
		return 0
	}
	t := s.Table()
	lands := 0
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjZone(ob) == kop.ZoneBattlefield && s.Controller(ob) == seat && !s.Tapped(ob) &&
			t.Cards[s.ObjCard(ob)].Types&kop.TLand != 0 {
			lands++
		}
	}
	for o := 0; o < s.NumObjs(); o++ {
		ob := dirac.Obj(o)
		if s.ObjZone(ob) != kop.ZoneHand || s.ObjOwner(ob) != seat {
			continue
		}
		def := &t.Cards[s.ObjCard(ob)]
		if def.Types&kop.TLand != 0 || (def.Types&kop.TInstant == 0 && def.Keywords&kop.KwFlash == 0) {
			continue
		}
		if manaValue(&def.Cost) <= lands {
			return sr.Opts.HoldBonus
		}
	}
	return 0
}

// manaValue is a printed cost's mana value ({X} counts 0, a hybrid pip 1).
func manaValue(c *kop.ManaCost) int {
	n := int(c.Generic)
	for _, p := range c.Pips {
		n += int(p)
	}
	for _, h := range c.Hyb {
		if h != 0 {
			n++
		}
	}
	return n
}
