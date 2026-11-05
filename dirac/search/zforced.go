package search

import "math"

// Forced playouts and policy target pruning (gorge bots/azmcts forced.go;
// KataGo, arXiv:1902.10565 section 3.1), Options.ForcedK. During the search every
// root child c the world offers is guaranteed n_forced(c) = sqrt(k * P(c) *
// N) visits, N the root children's visits so far and P the tree's root
// prior (root noise included): a child below that is selected before PUCT
// is asked. The forced visits are then taken back out of the policy target
// (pruneTarget), so the network is not taught to play the moves it was only
// forced to look at. In-walk points are never forced. Under CachedWorlds
// each world's tree forces its own root against its own visits, while
// pruning reads the merged root (as azmcts). Allocation-free; with ForcedK
// 0 the walk's selection is selectEdge's, unchanged.

// forcedVisits is n_forced = sqrt(k * prior * total).
func forcedVisits(k, prior float64, total int32) float64 {
	return math.Sqrt(k * prior * float64(total))
}

// selectAt is the walk's selection at node ni (walk, nodeCache.simulate):
// selectEdge, or at the root (node 0) under ForcedK the forced-playout rule
// (azmcts selectAt). It lives in its own file, out of line, so the hot
// files keep their code layout.
func (sr *Searcher) selectAt(ni int32, pt *point) int32 {
	if ni == 0 && (sr.Opts.ForcedK > 0 || sr.i42.shOn) {
		if sr.i42.shOn {
			return sr.shSelect(ni, pt) // zzzi42.go: RootSH
		}
		return sr.t.selectForced(ni, pt, &sr.Opts, &sr.St)
	}
	return sr.t.selectEdge(ni, pt, &sr.Opts)
}

// selectForced is azmcts selectForced: among the children pt offers whose
// visits are below forcedVisits, the one PUCT scores highest (ties to the
// earlier child, as selectEdge's); with none below, selectEdge's choice. A
// forced choice that differs from selectEdge's is counted
// (Stats.ForcedPlayouts).
func (t *tree) selectForced(ni int32, pt *point, o *Options, st *Stats) int32 {
	plain := t.selectEdge(ni, pt, o)
	nd := &t.nodes[ni]
	var total int32
	for e := nd.first; e >= 0; e = t.edges[e].sib {
		total += t.edges[e].n
	}
	parentQ := nd.q()
	if pt.opp {
		parentQ = 1 - parentQ
	}
	best := int32(-1)
	bestScore := math.Inf(-1)
	for e := nd.first; e >= 0; e = t.edges[e].sib {
		kid := &t.edges[e]
		if kid.dead || !pt.has(kid.key) || float64(kid.n) >= forcedVisits(o.ForcedK, kid.prior, total) {
			continue
		}
		q := parentQ - o.FPU
		if o.AbsoluteUnvisitedQ {
			q = o.UnvisitedQ
		}
		if kid.n > 0 {
			q = kid.w / float64(kid.n)
			if pt.opp {
				q = 1 - q
			}
		}
		if s := puctScore(q, kid.prior, kid.avail+1, kid.n, o.CPUCT); best < 0 || s > bestScore {
			best, bestScore = e, s
		}
	}
	if best < 0 {
		return plain
	}
	if best != plain {
		st.ForcedPlayouts++
	}
	return best
}

// pruneTarget is azmcts pruneTarget into res.Target (HasTarget): c*, the
// most-visited root child (ties to the lower index), keeps its visits;
// every other visited child c gives back up to n_forced(c) visits (N the
// total), but only while its PUCT value with the reduced count -- N_avail
// = avail+1, as selectEdge would score it next -- stays below c*'s with
// c*'s own visits. A child left with one visit or none is dropped (0). The
// prior is the tree's (the noisy one under Noise). Counts go to
// sr.St (PrunedVisits, PrunedChildren), which Search copies into res.Stats.
func (sr *Searcher) pruneTarget(res *Result) {
	prior := &sr.rootPt.prior
	if sr.Opts.Noise {
		prior = &sr.noisy
	}
	res.Target = res.Visits
	res.HasTarget = true
	best := 0
	var total int32
	for i := 0; i < res.N; i++ {
		total += res.Visits[i]
		if res.Visits[i] > res.Visits[best] {
			best = i
		}
	}
	if total == 0 {
		return
	}
	c := sr.Opts.CPUCT
	bestScore := puctScore(res.Q[best], prior[best], res.Avail[best]+1, res.Visits[best], c)
	for i := 0; i < res.N; i++ {
		n := res.Visits[i]
		if i == best || n == 0 {
			continue
		}
		lo := ceilNonNeg(float64(n) - forcedVisits(sr.Opts.ForcedK, prior[i], total))
		m := n
		for m > lo && puctScore(res.Q[i], prior[i], res.Avail[i]+1, m-1, c) < bestScore {
			m--
		}
		if m <= 1 {
			m = 0
			sr.St.PrunedChildren++
		}
		sr.St.PrunedVisits += int(n - m)
		res.Target[i] = m
	}
}

// ceilNonNeg is max(0, ceil(x)) for the visit counts pruneTarget handles
// (no math.Ceil: a new stdlib reference shifts dirac's hot code layout
// and costs a few percent).
func ceilNonNeg(x float64) int32 {
	if !(x > 0) {
		return 0
	}
	c := int32(x)
	if float64(c) < x {
		c++
	}
	return c
}
