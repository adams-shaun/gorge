package azmcts

import "math"

// Forced playouts and policy target pruning (KataGo, arXiv:1902.10565 §3.1;
// DraftZero research memo B/C6), Options.ForcedK. During the search every
// root child c the world offers is guaranteed n_forced(c) = sqrt(k * P(c) *
// N) visits, N the root children's visits so far and P the tree's root prior
// (root noise included): a child below that is selected before PUCT is asked.
// So a low-prior candidate keeps being looked at when the prior is sharp and
// cpuct small (DraftZero's 0.25). The forced visits are then taken back out
// of the policy target (pruneTarget), so the network is not taught to play
// the moves it was only forced to look at. In-walk points are never forced.

// forcedVisits is n_forced = sqrt(k * prior * total): the visits a root
// child of that prior is guaranteed once the root's children hold total.
func forcedVisits(k, prior float64, total int) float64 {
	return math.Sqrt(k * prior * float64(total))
}

// selectForced is the root's selection under Options.ForcedK: among the
// children pt offers whose visits are below forcedVisits, the one PUCT
// scores highest (ties to the lower index, as selectEdge's); with none
// below, selectEdge's choice. A forced choice that differs from
// selectEdge's is counted (Stats.ForcedPlayouts).
func selectForced(nd *node, pt *Point, opts Options, st *Stats) *edge {
	plain := selectEdge(nd, pt, opts)
	total := 0
	for _, kid := range nd.kids {
		total += kid.n
	}
	parentQ := nd.q()
	if pt.Opp {
		parentQ = 1 - parentQ
	}
	var best *edge
	bestScore := math.Inf(-1)
	for _, kid := range nd.kids {
		if !hasKey(pt.Keys, kid.key) || float64(kid.n) >= forcedVisits(opts.ForcedK, kid.prior, total) {
			continue
		}
		if s := edgeScore(kid, parentQ, pt.Opp, opts); best == nil || s > bestScore {
			best, bestScore = kid, s
		}
	}
	if best == nil {
		return plain
	}
	if best != plain {
		st.ForcedPlayouts++
	}
	return best
}

// policyTarget is the visits a record of r carries: the pruned Target under
// Options.ForcedK, else the raw Visits.
func (r Result) policyTarget() []int {
	if r.Target != nil {
		return r.Target
	}
	return r.Visits
}

// pruneTarget is KataGo's policy target pruning of a forced search's root:
// visits, q, avail and prior parallel the root's candidates (prior is the
// tree's, the one PUCT read). c*, the most-visited child (ties to the lower
// index, choose's argmax), keeps its visits. Every other visited child c
// gives back up to n_forced(c) visits (N the total visits), but only while
// its PUCT value with the reduced count -- puctScore with N_avail = avail+1,
// as selectEdge would score it next -- stays below c*'s with c*'s own
// visits: never fewer visits than PUCT itself would have given c. A child
// left with one visit or none is dropped from the target (0). The result is
// a new slice; visits is unchanged. The counts go to st (PrunedVisits,
// PrunedChildren).
func pruneTarget(visits []int, q []float64, avail []int, prior []float64, opts Options, st *Stats) []int {
	target := append([]int(nil), visits...)
	best, total := 0, 0
	for i, v := range visits {
		total += v
		if v > visits[best] {
			best = i
		}
	}
	if total == 0 {
		return target
	}
	bestScore := puctScore(q[best], prior[best], avail[best]+1, visits[best], opts.CPUCT)
	for c, n := range visits {
		if c == best || n == 0 {
			continue
		}
		lo := max(0, int(math.Ceil(float64(n)-forcedVisits(opts.ForcedK, prior[c], total))))
		m := n
		for m > lo && puctScore(q[c], prior[c], avail[c]+1, m-1, opts.CPUCT) < bestScore {
			m--
		}
		if m <= 1 {
			m = 0
			st.PrunedChildren++
		}
		st.PrunedVisits += n - m
		target[c] = m
	}
	return target
}
