package search

import "github.com/adams-shaun/gorge/dirac"

// Max^n at the opponents' points (Options.MaxN; gorge bots/azmcts's
// maxNApplies, leafVec and splitLeafVec). Every leaf's value vector is
// written to sr.vec, indexed by relSeat; the tree backs it up beside the
// scalar (tree.nwv, tree.ewv) and an opponent's point selects on its own
// entry (tree.selectEdge).

// maxNApplies reports whether Options.MaxN takes effect at the root: asked
// for, with the opponents in the tree and the vectors allocated, without
// NodeCache or Batch (Validate refuses both: their backups carry the scalar
// alone), and more than two living seats. With two the vector adds nothing
// and the scalar path runs unchanged.
func (sr *Searcher) maxNApplies() bool {
	o := &sr.Opts
	if !o.MaxN || !o.OpponentNodes || sr.t.nwv == nil || o.NodeCache > 0 || o.Batch > 0 {
		return false
	}
	living := 0
	for p := uint8(0); p < sr.root.NumSeats(); p++ {
		if !sr.root.FeatLost(p) {
			living++
		}
	}
	return living > 2
}

// overVec writes the value vector of a game that is over to sr.vec:
// one-hot on its winner, a draw uniform over the seats still in the game.
func (sr *Searcher) overVec(s *dirac.State) {
	sr.vec = [dirac.MaxSeats]float64{}
	n := s.NumSeats()
	if w := s.Winner(); w >= 0 {
		sr.vec[relSeat(uint8(w), sr.seat, n)] = 1
		return
	}
	living := 0
	for p := uint8(0); p < n; p++ {
		if !s.FeatLost(p) {
			living++
		}
	}
	for p := uint8(0); p < n; p++ {
		if !s.FeatLost(p) {
			sr.vec[relSeat(p, sr.seat, n)] = 1 / float64(living)
		}
	}
}

// liveVec writes the value vector of a leaf whose game plays on to sr.vec
// and returns the searching seat's entry, the value the scalar backups take:
// the evaluator's own vector (VecEvaluator, when its VecReporter, if any,
// covers the root's seats: sr.veOn), else its value at the searching seat
// (0 when it has lost, lost) with the rest split over the living opponents
// (splitVec).
func (sr *Searcher) liveVec(s *dirac.State, d *dirac.Decision, lost bool) float64 {
	if sr.veOn {
		sr.St.VecLeaves++
		sr.ve.ValueVec(s, d, sr.seat, &sr.vec)
		return sr.vec[0]
	}
	v := 0.0
	if !lost {
		v = float64(sr.Eval.Value(s, d, sr.seat))
	}
	splitVec(s, sr.seat, v, &sr.vec)
	return v
}

// splitVec is gorge's splitLeafVec: an evaluator that values only seat's
// position, v, leaves the rest, 1 - v, split evenly over the living
// opponents. A stand-in until the network has a value head per seat.
func splitVec(s *dirac.State, seat uint8, v float64, out *[dirac.MaxSeats]float64) {
	*out = [dirac.MaxSeats]float64{}
	n := s.NumSeats()
	out[0] = v
	opps := 0
	for p := uint8(0); p < n; p++ {
		if p != seat && !s.FeatLost(p) {
			opps++
		}
	}
	for p := uint8(0); p < n; p++ {
		if p != seat && !s.FeatLost(p) {
			out[relSeat(p, seat, n)] = (1 - v) / float64(opps)
		}
	}
}
