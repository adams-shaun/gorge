package azmcts

import (
	"context"
	"math"

	"github.com/adams-shaun/gorge/engine/rules"
)

// runFlat is a mulligan root's search (Options.MulliganWorlds,
// MulliganTurns): flat Monte Carlo over paired worlds. Each of worlds deals
// is played once per root candidate, every candidate from a copy of the same
// deal with the same future chance, and each walk lets both seats' bots play
// until the horizon (walkConfig.horizon) before its leaf is scored. A
// candidate's Q is its mean leaf over the deals it finished. Pairing the
// candidates on one deal is what lets a few dozen deals tell two hands
// apart: most of a leaf's spread is the deal (the opponent's cards, the
// draws), and it falls out of the comparison of candidate i with candidate
// 0 deal by deal. The second result is the move (flatMargin).
//
// Visits count each candidate's finished walks and Avail its attempted
// ones; RootValue is the chosen candidate's Q. ctx is the wall-clock
// bail-out, checked between deals, as RunTree's between simulations.
func runFlat(ctx context.Context, root *Point, cands []cand, src WorldSource, cfg *walkConfig, worlds int, seed uint64, st *Stats) (TreeResult, int) {
	n := len(root.Keys)
	res := TreeResult{Visits: make([]int, n), Q: make([]float64, n), Avail: make([]int, n), RootValue: 0.5}
	sums := make([]float64, n)
	// diff[i] and diff2[i] sum, over the deals where both finished, candidate
	// i's leaf minus candidate 0's and its square; paired[i] counts them.
	diff, diff2 := make([]float64, n), make([]float64, n)
	paired := make([]int, n)
	vals := make([]float64, n)
	done := make([]bool, n)
	var spare rules.Spare
	for w := 0; w < worlds; w++ {
		if ctx != nil && ctx.Err() != nil {
			st.DeadlineHits++
			break
		}
		base, err := src.World(w)
		if err != nil {
			for range n {
				st.Simulations++
				classify(st, err)
			}
			continue
		}
		chance := splitmix(seed ^ splitmix(uint64(w)+0x666c6174))
		for i := range n {
			st.Simulations++
			res.Avail[i]++
			v, err := flatWalk(base, cands[i].key, chance, &spare, cfg, st)
			done[i] = err == nil
			if err != nil {
				classify(st, err)
				continue
			}
			st.Completed++
			res.Visits[i]++
			sums[i] += v
			vals[i] = v
		}
		for i := 1; i < n; i++ {
			if done[0] && done[i] {
				d := vals[i] - vals[0]
				diff[i] += d
				diff2[i] += d * d
				paired[i]++
			}
		}
	}
	for i := range n {
		if res.Visits[i] > 0 {
			res.Q[i] = sums[i] / float64(res.Visits[i])
		}
	}
	// Candidate 0, the seat's own answer, stands unless another beats it on
	// the paired deals by more than flatMargin standard errors; among those
	// that do, the largest mean gain wins (ties to the lower index).
	choice, gain := 0, 0.0
	for i := 1; i < n; i++ {
		k := float64(paired[i])
		if paired[i] < 2 {
			continue
		}
		mean := diff[i] / k
		varSample := (diff2[i] - k*mean*mean) / (k - 1)
		se := math.Sqrt(max(varSample, 0) / k)
		if mean > flatMargin*se && mean > gain {
			choice, gain = i, mean
		}
	}
	if res.Visits[choice] > 0 {
		res.RootValue = res.Q[choice]
	}
	return res, choice
}

// flatMargin is how many standard errors of the paired difference a
// candidate must beat the seat's own answer by to replace it. Leaves vary a
// lot from deal to deal, and two answers a few turns cannot tell apart
// (keeping a fine seven, or a fine six) differ by noise alone; at 2 a
// candidate no better than the seat's answer overturns it about one time in
// forty-four per comparison (one-sided normal tail), so the rule's answer
// stands unless the deals show a real difference.
const flatMargin = 2.0

// flatWalk plays candidate key on a copy of base with future chance seeded
// by chance, then the walk's bots to the horizon, and returns the leaf.
func flatWalk(base World, key Key, chance uint64, spare *rules.Spare, cfg *walkConfig, st *Stats) (float64, error) {
	e := base.Engine.CloneHypotheticalInto(chance, spare)
	defer func() { *spare = e.Release() }()
	env, err := newEngineEnv(World{Engine: e, Observer: base.Observer, Hypothetical: true}, cfg)
	if err != nil {
		return 0, err
	}
	turn0 := env.Turn()
	if _, err := env.Play(key); err != nil {
		return 0, err
	}
	st.Plays++
	l := env.Leaf()
	if l.Err != nil {
		return 0, l.Err
	}
	if l.Terminal {
		st.Terminal++
	}
	if l.Capped {
		st.StepCapped++
	}
	st.LeafPlies += env.Plies()
	st.LeafEdges++
	st.LeafTurns += env.Turn() - turn0
	return l.V, nil
}
