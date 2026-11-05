package search

import "math"

// Batched leaf evaluation with virtual loss (Options.Batch, VirtualLoss).
//
// The serial tree (runTree/walk) runs one simulation at a time and calls
// Evaluator.Value at the leaf. Here up to Options.Batch simulations are in
// flight: each descends its own world (Searcher.worlds) until it reaches
// the value it would evaluate, the descent stops with the value pending,
// and the batch's pending leaves go to a BatchEvaluator.ValueBatch call
// together. While a descent is in flight, every edge on its path is
// marked (edge.inFlight) and selection subtracts
// VirtualLoss*inFlight/(n+1) from that edge's Q, so the next descent
// spreads to another child instead of piling onto a node whose value is
// not known yet. A descent may still pass through an in-flight node whose
// children exist (the node has no backed-up value yet: its n is 0 and its
// edges are the first world's); the virtual loss makes that rare, and the
// node's first real visit is whichever descent's value arrives first.
//
// Marks are per descent because a descent's backup is deferred: the tree
// mutation happens when its value arrives, in the batch's commit order.
// Batch 0 never enters this file: runTree/walk are untouched and their
// result is bit-identical.

// MaxBatch bounds Options.Batch (and the Searcher's preallocated worlds).
const MaxBatch = 8

// selectAtBatch is selectAt for the batched walk: the same selection plus
// the virtual-loss term for in-flight edges.
func (sr *Searcher) selectAtBatch(ni int32, pt *point) int32 {
	if ni == 0 && sr.Opts.ForcedK > 0 {
		return sr.t.selectForcedVL(ni, pt, &sr.Opts, &sr.St)
	}
	return sr.t.selectEdgeVL(ni, pt, &sr.Opts, sr.Opts.VirtualLoss)
}

// selectEdgeVL is selectEdge with the virtual-loss penalty: an edge with
// inFlight descents still on it scores Q - vl*inFlight/(n+1) (after the
// opponent-point flip, so the penalty always discourages the selector).
func (t *tree) selectEdgeVL(ni int32, pt *point, o *Options, vl float64) int32 {
	nd := &t.nodes[ni]
	parentQ := nd.q()
	if pt.opp {
		parentQ = 1 - parentQ
	}
	best := int32(-1)
	bestScore := math.Inf(-1)
	for e := nd.first; e >= 0; e = t.edges[e].sib {
		kid := &t.edges[e]
		if kid.dead || !pt.has(kid.key) {
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
		if vl > 0 && kid.inFlight > 0 {
			q -= vl * float64(kid.inFlight) / float64(kid.n+1)
		}
		if s := puctScore(q, kid.prior, kid.avail+1, kid.n, o.CPUCT); best < 0 || s > bestScore {
			best, bestScore = e, s
		}
	}
	return best
}

// selectForcedVL is selectForced with selectEdgeVL's scoring (the forced
// threshold counts committed visits, not in-flight ones).
func (t *tree) selectForcedVL(ni int32, pt *point, o *Options, st *Stats) int32 {
	vl := o.VirtualLoss
	plain := t.selectEdgeVL(ni, pt, o, vl)
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
		if vl > 0 && kid.inFlight > 0 {
			q -= vl * float64(kid.inFlight) / float64(kid.n+1)
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

// runTreeBatch is runTree with Options.Batch simulations in flight. The
// tree, root and statistics are the serial path's; only the order of the
// leaf values' arrivals differs (a batch's descents commit together).
func (sr *Searcher) runTreeBatch(sims, wi int, fixed bool) {
	t := &sr.t
	t.reset()
	top := t.newNode(&sr.rootPt)
	sr.noiseRoot(top)
	sr.cacheOn = false
	n := sr.Opts.Batch
	if n > MaxBatch {
		n = MaxBatch
	}
	for base := 0; base < sims; base += n {
		end := base + n
		if end > sims {
			end = sims
		}
		sr.batchN = 0
		for k := 0; k < end-base; k++ {
			sim := base + k
			sr.St.Simulations++
			w := &sr.worlds[k]
			sr.newWorld(w, sim, wi, fixed)
			pt := sr.worldRoot(w)
			if pt == nil {
				continue
			}
			if t.nodes[0].n == 0 {
				v := sr.rootV
				if !sr.haveRootV {
					v, _ = sr.leaf(w)
				}
				t.nodes[0].n, t.nodes[0].w = 1, v
			}
			sr.walkBatch(k, w, pt)
		}
		if sr.batchN == 0 {
			continue
		}
		sr.be.ValueBatch(sr.bstates[:sr.batchN], sr.bdecs[:sr.batchN], sr.seat, sr.bout[:sr.batchN])
		for j := 0; j < sr.batchN; j++ {
			k := int(sr.beval[j])
			v := float64(sr.bout[j])
			if sr.Opts.HoldBonus != 0 {
				v += sr.holdBonus(sr.bstates[j])
			}
			if nd := sr.bnode[k]; nd >= 0 {
				t.nodes[nd].n++
				t.nodes[nd].w += v
			}
			sr.commitBatch(k, int(sr.bdepth[k]), int(sr.bnm[k]), v)
			sr.St.Completed++
		}
		sr.batchN = 0
	}
}

// pending defers descent k's leaf value: node is the expansion it created
// (-1 for a step-capped leaf, which creates none), w the world holding
// the position to value.
func (sr *Searcher) pending(k int, node int32, w *world, depth, nm int) {
	j := sr.batchN
	sr.batchN++
	sr.bnode[k] = node
	sr.beval[j] = int32(k)
	sr.bdepth[k], sr.bnm[k] = int32(depth), int32(nm)
	sr.bstates[j], sr.bdecs[j] = &w.s, &w.d
}

// commitBatch backs v up descent k's recorded path and clears its
// in-flight marks (the same tree mutation as commit, plus the counter
// decrement).
func (sr *Searcher) commitBatch(k, depth, nm int, v float64) {
	t := &sr.t
	for i := 0; i < depth; i++ {
		nd := &t.nodes[sr.bpathN[k][i]]
		nd.n++
		nd.w += v
		e := &t.edges[sr.bpathE[k][i]]
		e.n++
		e.w += v
		if e.inFlight > 0 {
			e.inFlight--
		}
	}
	for i := 0; i < nm; i++ {
		t.edges[sr.bmarks[k][i]].avail++
	}
}

// clearInFlight drops descent k's marks without backing a value up (the
// walk ended with no leaf to value).
func (sr *Searcher) clearInFlight(k, depth int) {
	t := &sr.t
	for i := 0; i < depth; i++ {
		if e := &t.edges[sr.bpathE[k][i]]; e.inFlight > 0 {
			e.inFlight--
		}
	}
}

// walkBatch is walk for descent k in world w: it stops with the value
// pending instead of evaluating it, and records the path and availability
// marks for the deferred commit. The tree mutations before the leaf
// (edges added, a node expanded, an edge pruned) are the serial walk's.
func (sr *Searcher) walkBatch(k int, w *world, pt *point) {
	t := &sr.t
	pn, pe, mk := &sr.bpathN[k], &sr.bpathE[k], &sr.bmarks[k]
	ni := int32(0)
	depth, nm, unavail, cur := 0, 0, 0, 0
	for {
		nd := &t.nodes[ni]
		for e := nd.first; e >= 0; e = t.edges[e].sib {
			if pt.has(t.edges[e].key) {
				if nm < maxMarks {
					mk[nm] = e
					nm++
				}
			} else {
				unavail++
			}
		}
		for j := 0; j < pt.n; j++ {
			if t.child(ni, pt.keys[j]) < 0 {
				if e := t.addEdge(ni, pt.keys[j], pt.prior[j]); e >= 0 && nm < maxMarks {
					mk[nm] = e
					nm++
				}
			}
		}
		sel := sr.selectAtBatch(ni, pt)
		if sel < 0 {
			if depth > 0 {
				t.edges[pe[depth-1]].dead = true
				sr.St.NoOps++
			}
			sr.clearInFlight(k, depth)
			return
		}
		if pt.opp {
			sr.St.OppPoints++
		}
		if depth >= maxDepth {
			sr.St.TreeFull++
			sr.clearInFlight(k, depth)
			return
		}
		pn[depth], pe[depth] = ni, sel
		depth++
		ed := &t.edges[sel]
		ed.inFlight++
		replay, steps0 := ed.next >= 0 || ed.n > 0, sr.St.EnvSteps
		nextPt := &sr.pts[cur]
		cur ^= 1
		ok, undone := sr.play(w, pt, pt.index(ed.key), nextPt)
		if undone {
			ed.dead = true
			sr.St.NoOps++
			sr.clearInFlight(k, depth)
			return
		}
		sr.St.Plays++
		if replay {
			sr.St.ReplayPlays++
			sr.St.ReplaySteps += sr.St.EnvSteps - steps0
		}
		if !ok {
			v, term := sr.leaf(w)
			if term {
				sr.St.Terminal++
			}
			if w.capped {
				sr.St.StepCapped++
			}
			sr.St.LeafPlies += w.plies
			sr.St.LeafEdges += depth
			sr.St.Unavailable += unavail
			if term {
				sr.commitBatch(k, depth, nm, v)
				sr.St.Completed++
			} else {
				sr.pending(k, -1, w, depth, nm)
			}
			return
		}
		if ed.next < 0 {
			nn := t.newNode(nextPt)
			if nn < 0 {
				sr.St.TreeFull++
				sr.clearInFlight(k, depth)
				return
			}
			ed = &t.edges[sel]
			ed.next = nn
			sr.St.Expanded++
			if nextPt.opp {
				sr.St.OppExpanded++
			}
			sr.St.LeafPlies += w.plies
			sr.St.LeafEdges += depth
			sr.St.Unavailable += unavail
			sr.pending(k, nn, w, depth, nm)
			return
		}
		ni, pt = ed.next, nextPt
	}
}
