package search

import (
	"math"

	"github.com/adams-shaun/gorge/dirac"
)

// The tree is gorge bots/azmcts/tree.go's PUCT tree over preallocated,
// index-linked node and edge arrays (no pointers, no allocation after
// Init): ISMCTS availability, first-play urgency, opponent points read as
// 1 - Q, one expansion per simulation, value backed up the path.

// Key is a candidate's semantic identity at a node (azmcts Key): equal keys
// name the same action in every world.
type Key uint64

// MaxCands bounds a point's candidates.
const MaxCands = 64

// point is one searched decision as the tree sees it (azmcts Point).
type point struct {
	n   int
	opp bool
	// seat is the acting seat relative to the searching seat in turn order
	// (relSeat; 0 at the searching seat's own points): the value-vector
	// entry an opponent's point maximises under Options.MaxN.
	seat  uint8
	keys  [MaxCands]Key
	prior [MaxCands]float64
	// acts[i] is dirac action sequence key i plays (a declaration is
	// several picks); off[i]..off[i+1] index acts.
	off  [MaxCands + 1]uint16
	acts [maxActs]uint32
}

const maxActs = 512

func (pt *point) has(k Key) bool {
	for i := 0; i < pt.n; i++ {
		if pt.keys[i] == k {
			return true
		}
	}
	return false
}

func (pt *point) index(k Key) int {
	for i := 0; i < pt.n; i++ {
		if pt.keys[i] == k {
			return i
		}
	}
	return -1
}

type node struct {
	n     int32
	w     float64
	first int32 // first child edge, -1 none
	last  int32
	opp   bool
	snap  int32 // node cache: stored state index, -1 none
	plies int32 // engine clock at the node's point (node cache)
}

type edge struct {
	key   Key
	prior float64
	n     int32
	w     float64
	avail int32
	next  int32 // child node, -1 none
	sib   int32 // next sibling, -1 none
	// inFlight counts the batched descents (Options.Batch) whose path
	// currently crosses this edge; selection subtracts Options.VirtualLoss
	// for it (batch.go). Always 0 on the serial path.
	inFlight int32
	// node cache: an edge whose walk ended keeps its leaf
	ended   bool
	endTerm bool
	endCap  bool
	endV    float64
	// dead: playing the edge undid itself (dirac State.Undos: a cast
	// taken back), a no-op that would loop; never selected again
	dead bool
}

func (nd *node) q() float64 {
	if nd.n == 0 {
		return 0.5
	}
	return nd.w / float64(nd.n)
}

// puctScore is Q + c * P * sqrt(N_avail) / (1 + N) (azmcts puctScore).
func puctScore(q, prior float64, avail, n int32, c float64) float64 {
	return q + c*prior*math.Sqrt(float64(avail))/float64(1+n)
}

// tree is the preallocated node/edge store.
type tree struct {
	nodes []node
	edges []edge
	// nwv and ewv parallel nodes and edges under Options.MaxN (nil without
	// it, so the option off costs nothing): each node's and edge's backed-up
	// value vector (the leaf vectors summed as w sums the values), indexed
	// by relSeat. vec is set per search when max^n is in effect
	// (maxNApplies); selection reads the vectors only then.
	nwv  [][dirac.MaxSeats]float64
	ewv  [][dirac.MaxSeats]float64
	vec  bool
	nn   int32
	ne   int32
	full bool // an expansion or edge append was refused (capacity)
}

func (t *tree) reset() {
	t.nn, t.ne, t.full = 0, 0, false
}

func (t *tree) newNode(pt *point) int32 {
	if int(t.nn) >= len(t.nodes) {
		t.full = true
		return -1
	}
	i := t.nn
	t.nn++
	t.nodes[i] = node{first: -1, last: -1, opp: pt.opp, snap: -1}
	if t.nwv != nil {
		t.nwv[i] = [dirac.MaxSeats]float64{}
	}
	for j := 0; j < pt.n; j++ {
		t.addEdge(i, pt.keys[j], pt.prior[j])
	}
	return i
}

func (t *tree) addEdge(ni int32, k Key, prior float64) int32 {
	if int(t.ne) >= len(t.edges) {
		t.full = true
		return -1
	}
	e := t.ne
	t.ne++
	t.edges[e] = edge{key: k, prior: prior, next: -1, sib: -1}
	if t.ewv != nil {
		t.ewv[e] = [dirac.MaxSeats]float64{}
	}
	nd := &t.nodes[ni]
	if nd.last < 0 {
		nd.first = e
	} else {
		t.edges[nd.last].sib = e
	}
	nd.last = e
	return e
}

func (t *tree) child(ni int32, k Key) int32 {
	for e := t.nodes[ni].first; e >= 0; e = t.edges[e].sib {
		if t.edges[e].key == k {
			return e
		}
	}
	return -1
}

// vecSeat is the value-vector entry selection reads at pt: the acting
// opponent's relative seat at its point when max^n is in effect (tree.vec),
// else -1 (the scalar, read as 1 - Q at an opponent's point).
func (t *tree) vecSeat(pt *point) int {
	if t.vec && pt.opp {
		return int(pt.seat)
	}
	return -1
}

// selectEdge is azmcts selectEdge: PUCT over the children this world
// offers; an unvisited child's Q is the parent's minus FPU (or the
// absolute UnvisitedQ); at an opponent's point every Q reads 1 - Q, or under
// max^n (vecSeat) the opponent's own entry of the value vector, W_r / N;
// strict > keeps the earlier child on a tie.
func (t *tree) selectEdge(ni int32, pt *point, o *Options) int32 {
	nd := &t.nodes[ni]
	parentQ := nd.q()
	r := t.vecSeat(pt)
	switch {
	case r >= 0:
		parentQ = 0.5
		if nd.n > 0 {
			parentQ = t.nwv[ni][r] / float64(nd.n)
		}
	case pt.opp:
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
			switch {
			case r >= 0:
				q = t.ewv[e][r] / float64(kid.n)
			case pt.opp:
				q = 1 - q
			}
		}
		if s := puctScore(q, kid.prior, kid.avail+1, kid.n, o.CPUCT); best < 0 || s > bestScore {
			best, bestScore = e, s
		}
	}
	return best
}

// addVec adds vec to node ni's and edge e's backed-up vectors (max^n).
func (t *tree) addVec(ni, e int32, vec *[dirac.MaxSeats]float64) {
	for r, x := range vec {
		t.nwv[ni][r] += x
		t.ewv[e][r] += x
	}
}
