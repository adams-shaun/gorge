package neteval

import (
	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/dirac"
)

// The K-way value head (dzg's value_k): K softmax probabilities read off the
// value trunk, components 0..K-2 the relative seats from the viewer in turn
// order and component K-1 that no seat wins. Only the gonet backend runs it
// (LoadWeights routes such a network there), so only a gonet-backed Net
// reports a vector.

// ValueK is the width of the weights' K-way value head, 0 for a scalar
// network (or a netsimd-backed one, which has no such head).
func (w *Weights) ValueK() int {
	if w.gn != nil {
		return w.gn.ValueK()
	}
	return 0
}

// HasValueVec is search.VecReporter: the network has a value vector for a
// game of seats seats when its K-way head has a component for every seat
// and the no-winner outcome (K >= seats+1). The search falls back to Value
// and its even split otherwise.
func (n *Net) HasValueVec(seats uint8) bool { return len(n.vk) >= int(seats)+1 }

// ValueVec is search.VecEvaluator (Options.MaxN): the head's probabilities
// at s (pending decision d) from seat's view, the view Value reads, mapped
// onto the search's vector as gorge's netLeafVec (bots/azmcts) maps them: a
// living seat's entry is its win component plus the no-winner probability
// divided evenly over the living seats, and a lost seat's entry is 0, so
// the entries sum to the head's mass on the living seats and the draw.
// Evaluations are cached beside Value's under their own key. It allocates
// nothing; called without HasValueVec for s's seats it panics.
func (n *Net) ValueVec(s *dirac.State, d *dirac.Decision, seat uint8, out *[dirac.MaxSeats]float64) {
	seats := s.NumSeats()
	if !n.HasValueVec(seats) {
		panic("neteval: ValueVec without a K-way value head covering the game's seats")
	}
	k := len(n.vk)
	var key uint64
	if n.Keys == KeyContent {
		key = hu(n.valueKey(s, d, seat), 0x766563)
	} else {
		n.state(s, d, seat)
		key = hu(n.stateHash(), 0x766563)
	}
	var p []float32
	if e, hit := n.lookup(key); hit && int(e.n) == k {
		n.Hits++
		p = e.logits[:k]
	} else {
		if n.Keys == KeyContent {
			n.state(s, d, seat)
		}
		n.w.gn.EvalValueVec(&n.pst, n.vk)
		gonet.Softmax32(n.vk)
		n.Evals++
		p = n.vk
		if e != nil && k <= cacheOpts {
			e.key, e.ok, e.n, e.v = key, true, uint16(k), p[0]
			copy(e.logits[:k], p)
		}
	}
	*out = [dirac.MaxSeats]float64{}
	living := 0
	for q := uint8(0); q < seats; q++ {
		if !s.FeatLost(q) {
			living++
		}
	}
	draw := float64(p[k-1]) / float64(max(living, 1))
	for q := uint8(0); q < seats; q++ {
		if s.FeatLost(q) {
			continue
		}
		r := (q + seats - seat) % seats
		out[r] = float64(p[r]) + draw
	}
}
