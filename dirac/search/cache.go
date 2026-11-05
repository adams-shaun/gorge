package search

// The node state cache (azmcts nodecache.go) for a FIXED world (a
// CachedWorlds tree): a walk's state is a function of the keys it played,
// so a simulation resumes from the state stored at the deepest stored node
// of its path. Exact: the tree sees the same points, leaves and selections
// as without it; only the cost counters change. Stored: the root's state;
// a newly expanded node's state while there is room; an unstored node a
// walk passes through while there is room or by evicting the least-visited
// stored node (never the root) with strictly fewer visits; an ended edge's
// leaf.
type nodeCache struct {
	max    int
	worlds []world
	points []point
	owner  []int32 // node index per slot
	n      int
}

func (c *nodeCache) init(max int) {
	c.max = max
	c.worlds = make([]world, max)
	c.points = make([]point, max)
	c.owner = make([]int32, max)
}

func (c *nodeCache) reset() { c.n = 0 }

func (c *nodeCache) store(sr *Searcher, slot int, ni int32, pt *point) {
	c.worlds[slot].copyFrom(&sr.w)
	c.points[slot] = *pt
	c.owner[slot] = ni
	sr.t.nodes[ni].snap = int32(slot)
	sr.St.NodeSaves++
}

// offer stores sr.w at node ni if there is room or a less-visited stored
// node to evict (final: a new leaf never evicts).
func (c *nodeCache) offer(sr *Searcher, ni int32, pt *point, final bool) {
	if c.n < c.max {
		c.store(sr, c.n, ni, pt)
		c.n++
		return
	}
	if final {
		return
	}
	victim := -1
	for i := 1; i < c.n; i++ { // slot 0 is the root: never evicted
		if victim < 0 || sr.t.nodes[c.owner[i]].n < sr.t.nodes[c.owner[victim]].n {
			victim = i
		}
	}
	if victim < 0 || sr.t.nodes[c.owner[victim]].n >= sr.t.nodes[ni].n {
		return
	}
	sr.t.nodes[c.owner[victim]].snap = -1
	sr.St.NodeEvicts++
	c.store(sr, victim, ni, pt)
}

// simulate is one cached simulation in fixed world wi.
func (c *nodeCache) simulate(sr *Searcher, wi int) bool {
	t := &sr.t
	if t.nodes[0].snap < 0 {
		sr.newWorld(&sr.w, 0, wi, true)
		pt := sr.worldRoot(&sr.w)
		if pt == nil {
			return false
		}
		c.store(sr, 0, 0, pt)
		if c.n == 0 {
			c.n = 1
		}
	}
	if t.nodes[0].n == 0 {
		sr.w.copyFrom(&c.worlds[0])
		v := sr.rootV
		if !sr.haveRootV {
			v, _ = sr.leaf(&sr.w)
		}
		t.nodes[0].n, t.nodes[0].w = 1, v
	}
	ni := int32(0)
	pt := &c.points[0]
	depth, nm, unavail := 0, 0, 0
	for {
		nm0, unavail0 := nm, unavail
		nd := &t.nodes[ni]
		for e := nd.first; e >= 0; e = t.edges[e].sib {
			if pt.has(t.edges[e].key) {
				if nm < maxMarks {
					sr.marks[nm] = e
					nm++
				}
			} else {
				unavail++
			}
		}
		for j := 0; j < pt.n; j++ {
			if t.child(ni, pt.keys[j]) < 0 {
				if e := t.addEdge(ni, pt.keys[j], pt.prior[j]); e >= 0 && nm < maxMarks {
					sr.marks[nm] = e
					nm++
				}
			}
		}
		fp0 := sr.St.ForcedPlayouts
		sel := sr.selectAt(ni, pt) // forced.go
		if pt.opp {
			sr.St.OppPoints++
		}
		sr.pathN[depth], sr.pathE[depth] = ni, sel
		depth++
		ed := &t.edges[sel]
		sr.St.Plays++
		if ed.ended {
			sr.St.ReplayPlays++
			if ed.endTerm {
				sr.St.Terminal++
			}
			if ed.endCap {
				sr.St.StepCapped++
			}
			sr.commit(depth, nm, ed.endV)
			sr.St.Unavailable += unavail
			return true
		}
		if ed.next >= 0 && t.nodes[ed.next].snap >= 0 {
			sr.St.ReplayPlays++
			ni = ed.next
			pt = &c.points[t.nodes[ni].snap]
			continue
		}
		// resume from the deepest stored node and walk on from here
		if ni != 0 {
			sr.St.NodeResumes++
		}
		sr.w.copyFrom(&c.worlds[t.nodes[ni].snap])
		sr.w.s.SetLine(sr.nextLine())
		if sr.wr != nil {
			sr.wr.NewWorld()
		}
		// undo this node's bookkeeping: walk redoes it from the stored state
		depth--
		sr.St.Plays--
		sr.St.ForcedPlayouts = fp0
		if pt.opp {
			sr.St.OppPoints--
		}
		return sr.walk(ni, pt, depth, nm0, unavail0, 0)
	}
}
