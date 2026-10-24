package gonet

import (
	"fmt"
	"math"

	"github.com/adams-shaun/gorge/bots/internal/simdk"
	"github.com/adams-shaun/gorge/bots/policynet"
)

// The graph network's forward pass (dzg/models.py GraphNet), the second
// architecture a .dzgw file can carry. Everything structural is fixed; only
// the widths come from the header, so the same code reads a 128-wide research
// network and a wider one without a new loader.
//
// The graph, per state:
//
//	root -> players (me, opponent) -> zones (my battlefield and hand under me,
//	the opponent's battlefield under them, the stack under the root) -> cards
//	-> leaves (ONE LEAF PER IDENTITY ROW, table[row]·value through leaf_proj,
//	plus a Linear(raw) leaf); the root's other children are the DENSE leaf and
//	one leaf per sparse row.
//
// A bottom-up pass of local attention layers (card, zone, player, root) per
// `passes`, then pre-norm transformer layers over [root, me, opp, 4 zones,
// cards] and a final LayerNorm; value = MLP(root). An option's query is
// opt_in(o) + ea_proj(h_ea) + eb_proj(h_eb), where h_ea/h_eb are the referenced
// cards' FINAL tokens (or the learned none_a/none_b); it cross-attends over the
// final tokens and scores MLP([root ‖ q' ‖ h_ea ‖ h_eb]) with h_ea/h_eb
// UNPROJECTED.
//
// Numerics: every Linear is a batched GEMM over pre-packed weight panels
// (bots/internal/simdk: AVX-512 when the CPU has it, a scalar twin
// otherwise), LayerNorm and GELU are vector kernels too, and attention
// logits and softmax run in float64 (the reference uses fp32 with a
// log-sum-exp). Every loop has a fixed summation order, so a forward is
// deterministic on a given CPU and kernel choice.

// ForceScalar routes the graph forward's kernels through their scalar twins
// (tests compare the two paths; a benchmark measures the gap).
var ForceScalar bool

// gemm: Y[r] = W·X[r] + b for n contiguous rows, through the packed panels.
func (l *gnLinear) gemm(Y, X []float32, n int) {
	simdk.Gemm(Y, l.out, X, l.in, n, l.pk, !ForceScalar)
}

// rows: y[r] = LN(x[r]) for n rows of width d (y may be x).
func (nm *gnNorm) rows(y, x []float32, n, d int) {
	simd := !ForceScalar
	for r := 0; r < n; r++ {
		simdk.LayerNorm(y[r*d:(r+1)*d], x[r*d:(r+1)*d], nm.w, nm.b, nm.eps, simd)
	}
}

// Graph node types and edge types (GraphNet's ROOT..CARD and E_* constants).
const (
	gnNodeRoot = iota
	gnNodeMe
	gnNodeOpp
	gnNodeMyBF
	gnNodeOppBF
	gnNodeHand
	gnNodeStack
	gnNodeCard
	gnNumNodeTypes
)

const (
	gnEdgeIdent = iota
	gnEdgeRaw
	gnEdgeDense
	gnEdgeSparse
	gnEdgeCardZone
	gnEdgeZonePlayer
	gnEdgeStackRoot
	gnEdgePlayerRoot
	gnNumEdgeTypes
)

// gnFixed is GraphNet's N_FIXED: per state, the root, the two players and the
// four zone nodes, in that order, ahead of the cards.
const gnFixed = 7

// The zones under the players (GraphNet's zone_ids / zone_player buffers,
// non-persistent, so constants of the architecture rather than tensors): zone
// 0 (my battlefield) and zone 2 (my hand) under me, zone 1 (the opponent's
// battlefield) under the opponent. Zone 3, the stack, is a child of the root.
var (
	gnPlayerZones   = [3]int{0, 2, 1}
	gnPlayerOfZones = [3]int{0, 0, 1}
)

// gnLocalLayer is LocalLayer: pre-norm attention from each node to [itself ‖
// its children], grouped by parent, then a pre-norm FFN. kv is qkv's k and v
// rows alone (w and b a view of qkv's; its packed panels its own): a child
// contributes keys and values only.
type gnLocalLayer struct {
	n1, n2   gnNorm
	qkv, kv  gnLinear // d -> 3d; d -> 2d (rows d..3d of qkv)
	out      gnLinear // d -> d
	fc1, fc2 gnLinear // d -> ff -> d
}

// gnSABlock is _SABlock: a pre-norm transformer encoder layer.
type gnSABlock struct {
	n1, n2   gnNorm
	qkv, out gnLinear
	fc1, fc2 gnLinear
}

// gnGNN holds the graph network's weights, separate from Net's MLP fields so
// a Net is exactly one architecture.
type gnGNN struct {
	d, heads, ff, hh, dEmb, rawW int

	nodeType, edgeType []float32         // types×d
	ltab               []float32         // TableRows×d: leaf_proj(table[r]) (leaf_proj has no bias)
	rawLeaf, denseLeaf gnLinear          // rawW -> d, DenseWidth -> d
	local              [][4]gnLocalLayer // [pass][card, zone, player, root]
	blocks             []gnSABlock
	norm               gnNorm

	slots          []float32 // OptionSlotWidth×d
	htab           []float32 // TableRows×d: opt_in.hproj(table[r])
	odense         gnLinear  // OptionDenseWidth -> d
	bot            []float32 // d
	eaProj, ebProj gnLinear  // d -> d
	noneA, noneB   []float32 // the learned "no card" vectors (unprojected)

	crossNQ, crossNKV, crossN2 gnNorm
	crossQ, crossKV, crossOut  gnLinear
	crossFF1, crossFF2         gnLinear

	policy, value []gnLinear
}

// gnnScratch is the graph forward's reusable working set; a warm scratch
// makes a forward allocation-free.
type gnnScratch struct {
	leaves, card, zone, play, root, kids []float32
	leafSeg, kidSeg, segAll              []int
	heavy, qkv, kv, out, tmp, ln, hid    []float32
	x, zc, fit                           []float32
	sum                                  []float64
	o, ha, hb, q, q2, z, ha1, hb1        []float32
	raws, rl, od, ta, v1, s32, mx32      []float32
}

func takeF(p *[]float32, n int) []float32 {
	if cap(*p) < n {
		*p = make([]float32, n, n+n/4)
	}
	*p = (*p)[:n]
	return *p
}

func takeI(p *[]int, n int) []int {
	if cap(*p) < n {
		*p = make([]int, n, n+n/4)
	}
	*p = (*p)[:n]
	return *p
}

func takeD(p *[]float64, n int) []float64 {
	if cap(*p) < n {
		*p = make([]float64, n, n+n/4)
	}
	*p = (*p)[:n]
	return *p
}

// loadGNN interprets a .dzgw's tensors as GraphNet's weights. Every tensor is
// fetched by name at the shape GraphNet gives it, so a mismatch is an error
// rather than a silently misread weight.
func loadGNN(path string, h gnHeader, tensors map[string][]float32, shapes map[string][]int, rawW int, hashChecked bool) (*Net, error) {
	cfg := func(k string) int { return int(h.Config[k]) }
	d, e := cfg("d"), cfg("d_emb")
	heads, ff, passes, gl, hh := cfg("heads"), cfg("ff"), cfg("passes"), cfg("global_layers"), cfg("head_hidden")
	if d <= 0 || e <= 0 || heads <= 0 || ff <= 0 || passes <= 0 || gl < 0 || hh <= 0 {
		return nil, fmt.Errorf("gonet %s: gnn config: d=%d d_emb=%d heads=%d ff=%d passes=%d global_layers=%d head_hidden=%d", path, d, e, heads, ff, passes, gl, hh)
	}
	if d%heads != 0 {
		return nil, fmt.Errorf("gonet %s: d=%d is not a multiple of heads=%d", path, d, heads)
	}
	var errs []error
	get := func(name string, shape ...int) []float32 {
		t, ok := tensors[name]
		if !ok {
			errs = append(errs, fmt.Errorf("missing tensor %s", name))
			return make([]float32, prod(shape))
		}
		s := shapes[name]
		bad := len(s) != len(shape)
		for i := 0; !bad && i < len(s); i++ {
			bad = s[i] != shape[i]
		}
		if bad {
			errs = append(errs, fmt.Errorf("tensor %s: shape %v, want %v", name, s, shape))
			return make([]float32, prod(shape))
		}
		return t
	}
	lin := func(name string, in, out int, bias bool) gnLinear {
		l := gnLinear{in: in, out: out, w: get(name+".weight", out, in)}
		if bias {
			l.b = get(name+".bias", out)
		}
		l.pk = simdk.Pack(l.w, l.b, in, out)
		return l
	}
	norm := func(name string, n int) gnNorm {
		eps, ok := h.Eps[name]
		if !ok {
			errs = append(errs, fmt.Errorf("no layernorm_eps for %s", name))
		}
		return gnNorm{w: get(name+".weight", n), b: get(name+".bias", n), eps: eps}
	}
	head := func(name string, dIn, layers int) []gnLinear {
		var out []gnLinear
		in := dIn
		for i := 0; i <= layers; i++ {
			o := hh
			if i == layers {
				o = 1
			}
			out = append(out, lin(fmt.Sprintf("%s.%d", name, 2*i), in, o, true))
			in = o
		}
		return out
	}
	g := &gnGNN{d: d, heads: heads, ff: ff, hh: hh, dEmb: e, rawW: rawW,
		nodeType:  get("node_type.weight", gnNumNodeTypes, d),
		edgeType:  get("edge_type.weight", gnNumEdgeTypes, d),
		rawLeaf:   lin("raw_leaf", rawW, d, true),
		denseLeaf: lin("dense_leaf", policynet.DenseWidth, d, true),
		norm:      norm("norm", d),
		slots:     get("opt_in.slots.weight", gnSlotRow, d),
		odense:    lin("opt_in.dense", policynet.OptionDenseWidth, d, true),
		bot:       get("opt_in.bot", d),
		eaProj:    lin("ea_proj", d, d, true),
		ebProj:    lin("eb_proj", d, d, true),
		noneA:     get("none_a", d),
		noneB:     get("none_b", d),
		policy:    head("policy", 4*d, cfg("policy_layers")),
		value:     head("value", d, cfg("value_layers")),
	}
	table := get("table.weight", gnRows, e)
	leafProj := lin("leaf_proj", e, d, false)
	hproj := lin("opt_in.hproj", e, d, false)
	for p := 0; p < passes; p++ {
		var l [4]gnLocalLayer
		for i, name := range [4]string{"card", "zone", "player", "root"} {
			pre := fmt.Sprintf("local.%d.%s", p, name)
			qkv := lin(pre+".qkv", d, 3*d, true)
			kv := gnLinear{in: d, out: 2 * d, w: qkv.w[d*d:], b: qkv.b[d:]}
			kv.pk = simdk.Pack(kv.w, kv.b, d, 2*d)
			l[i] = gnLocalLayer{n1: norm(pre+".n1", d), n2: norm(pre+".n2", d), qkv: qkv, kv: kv,
				out: lin(pre+".out", d, d, true), fc1: lin(pre+".ff1", d, ff, true), fc2: lin(pre+".ff2", ff, d, true)}
		}
		g.local = append(g.local, l)
	}
	for i := 0; i < gl; i++ {
		p := fmt.Sprintf("blocks.%d", i)
		g.blocks = append(g.blocks, gnSABlock{n1: norm(p+".n1", d), qkv: lin(p+".qkv", d, 3*d, true),
			out: lin(p+".out", d, d, true), n2: norm(p+".n2", d), fc1: lin(p+".ff1", d, ff, true), fc2: lin(p+".ff2", ff, d, true)})
	}
	g.crossNQ, g.crossNKV, g.crossN2 = norm("cross.nq", d), norm("cross.nkv", d), norm("cross.n2", d)
	g.crossQ, g.crossKV, g.crossOut = lin("cross.q", d, d, true), lin("cross.kv", d, 2*d, true), lin("cross.out", d, d, true)
	g.crossFF1, g.crossFF2 = lin("cross.ff1", d, ff, true), lin("cross.ff2", ff, d, true)
	if len(errs) > 0 {
		return nil, fmt.Errorf("gonet %s: %v", path, errs)
	}
	// Both bias-free projections of the shared table, once: an identity or
	// sparse leaf, and an option's hashed bag, become row gathers.
	g.ltab = make([]float32, gnRows*d)
	leafProj.applyN(g.ltab, table, gnRows, true)
	g.htab = make([]float32, gnRows*d)
	hproj.applyN(g.htab, table, gnRows, true)
	p := &Net{path: path, hashChecked: hashChecked, d: d, ff: ff, dEmb: e, hh: hh, gnn: g}
	if CacheSize > 0 {
		p.cache = NewEvalCache(CacheSize)
	}
	p.pool.New = func() any { return new(gnScratch) }
	return p, nil
}

func prod(s []int) int {
	n := 1
	for _, v := range s {
		n *= v
	}
	return n
}

func addRow(dst, src []float32) {
	src = src[:len(dst)]
	for i := range dst {
		dst[i] += src[i]
	}
}

// leaf writes val·tab[r] + edge into dst (a row past the table is the edge alone).
func leaf(dst, tab []float32, r int, val float32, edge []float32) {
	d := len(dst)
	if r < 0 || r >= gnRows {
		copy(dst, edge)
		return
	}
	src := tab[r*d : (r+1)*d]
	for i := range dst {
		dst[i] = val*src[i] + edge[i]
	}
}

// ffn is x += ff2(gelu(ff1(n2(x)))) over n rows of x, in place.
func ffn(sc *gnnScratch, n2 *gnNorm, fc1, fc2 *gnLinear, x []float32, n, d int) {
	ln := takeF(&sc.ln, n*d)
	n2.rows(ln, x, n, d)
	hid := takeF(&sc.hid, n*fc1.out)
	fc1.gemm(hid, ln, n)
	simdk.Gelu(hid, !ForceScalar)
	tmp := takeF(&sc.tmp, n*d)
	fc2.gemm(tmp, hid, n)
	addRow(x[:n*d], tmp)
}

// localLayer is LocalLayer.forward for one state.
//
//	node  [S, d]  the parent nodes (updated in place)
//	child [E, d]  one row per edge
//	seg   [E]     each edge's parent index
//
// Each parent's query attends over {itself} ∪ {its children}.
func localLayer(g *gnGNN, l *gnLocalLayer, sc *gnnScratch, node, child []float32, seg []int, S, E int) {
	d, heads, dh := g.d, g.heads, g.d/g.heads
	heavy := takeF(&sc.heavy, max(S, E)*d)
	l.n1.rows(heavy, node, S, d)
	qkv := takeF(&sc.qkv, S*3*d)
	l.qkv.gemm(qkv, heavy, S)
	l.n1.rows(heavy, child, E, d)
	kv := takeF(&sc.kv, E*2*d)
	l.kv.gemm(kv, heavy, E)

	// logits: row r < S is node r's own key; row S+e is child e's
	simd := !ForceScalar
	total := S + E
	segAll := takeI(&sc.segAll, total)
	for i := 0; i < S; i++ {
		segAll[i] = i
	}
	copy(segAll[S:], seg[:E])
	score := takeF(&sc.s32, total*heads)
	for r := 0; r < total; r++ {
		p := segAll[r]
		var k []float32
		if r < S {
			k = qkv[r*3*d+d : r*3*d+2*d]
		} else {
			k = kv[(r-S)*2*d : (r-S)*2*d+d]
		}
		simdk.DotHeads(score[r*heads:(r+1)*heads], qkv[p*3*d:p*3*d+d], k, heads, dh, simd)
	}
	// segment softmax: every segment holds its own node, so none is empty
	inv := float32(1 / math.Sqrt(float64(dh)))
	mx := takeF(&sc.mx32, S*heads)
	for i := range mx {
		mx[i] = float32(math.Inf(-1))
	}
	for r := 0; r < total; r++ {
		b := segAll[r] * heads
		for h := 0; h < heads; h++ {
			v := score[r*heads+h] * inv
			score[r*heads+h] = v
			mx[b+h] = max(mx[b+h], v)
		}
	}
	for r := 0; r < total; r++ {
		b := segAll[r] * heads
		for h := 0; h < heads; h++ {
			score[r*heads+h] -= mx[b+h]
		}
	}
	simdk.Exp(score, simd)
	sum := takeD(&sc.sum, S*heads)
	clear(sum)
	for r := 0; r < total; r++ {
		b := segAll[r] * heads
		for h := 0; h < heads; h++ {
			sum[b+h] += float64(score[r*heads+h])
		}
	}
	// the weights in place, then every row's value into its parent
	for r := 0; r < total; r++ {
		b := segAll[r] * heads
		for h := 0; h < heads; h++ {
			score[r*heads+h] = float32(float64(score[r*heads+h]) / sum[b+h])
		}
	}
	out := takeF(&sc.out, S*d)
	clear(out)
	for r := 0; r < total; r++ {
		p := segAll[r]
		var v []float32
		if r < S {
			v = qkv[r*3*d+2*d : (r+1)*3*d]
		} else {
			v = kv[(r-S)*2*d+d : (r-S+1)*2*d]
		}
		simdk.AxpyHeads(out[p*d:(p+1)*d], v, score[r*heads:(r+1)*heads], heads, dh, simd)
	}
	tmp := takeF(&sc.tmp, S*d)
	l.out.gemm(tmp, out, S)
	addRow(node[:S*d], tmp)
	ffn(sc, &l.n2, &l.fc1, &l.fc2, node, S, d)
}

// globalPass runs the pre-norm transformer blocks over one state's tokens
// x [T, d] (every token real: one state needs no padding mask), then the final
// LayerNorm, in place.
func globalPass(g *gnGNN, sc *gnnScratch, x []float32, T int) {
	d, heads, dh := g.d, g.heads, g.d/g.heads
	inv := 1 / math.Sqrt(float64(dh))
	for bi := range g.blocks {
		b := &g.blocks[bi]
		ln := takeF(&sc.ln, T*d)
		b.n1.rows(ln, x, T, d)
		qkv := takeF(&sc.qkv, T*3*d)
		b.qkv.gemm(qkv, ln, T)
		out := takeF(&sc.out, T*d)
		attend(qkv, 3*d, qkv[d:], qkv[2*d:], 3*d, T, T, heads, dh, inv, out, d, sc)
		tmp := takeF(&sc.tmp, T*d)
		b.out.gemm(tmp, out, T)
		addRow(x[:T*d], tmp)
		ffn(sc, &b.n2, &b.fc1, &b.fc2, x, T, d)
	}
	g.norm.rows(x, x, T, d)
}

// attend is softmax(q·kᵀ·inv)·v per head for nq queries over nk keys. Row i
// of q starts at q[i*qs], row t of k and v at k[t*ks], v[t*ks]; the result's
// row i is out[i*os : i*os+heads*dh].
func attend(q []float32, qs int, k, v []float32, ks, nq, nk, heads, dh int, inv float64, out []float32, os int, sc *gnnScratch) {
	simd := !ForceScalar
	n := heads * dh
	s := takeF(&sc.s32, nk*heads) // [t][h]: the logits, then the weights
	m := takeF(&sc.mx32, heads)
	z := takeD(&sc.sum, heads)
	sc32 := float32(inv)
	for i := 0; i < nq; i++ {
		qi := q[i*qs : i*qs+n]
		for t := 0; t < nk; t++ {
			simdk.DotHeads(s[t*heads:(t+1)*heads], qi, k[t*ks:t*ks+n], heads, dh, simd)
		}
		for h := range m {
			m[h] = float32(math.Inf(-1))
			z[h] = 0
		}
		for t := 0; t < nk; t++ {
			st := s[t*heads : (t+1)*heads]
			for h := range st {
				st[h] *= sc32
				m[h] = max(m[h], st[h])
			}
		}
		for t := 0; t < nk; t++ {
			st := s[t*heads : (t+1)*heads]
			for h := range st {
				st[h] -= m[h]
			}
		}
		simdk.Exp(s, simd)
		for t := 0; t < nk; t++ {
			for h := 0; h < heads; h++ {
				z[h] += float64(s[t*heads+h])
			}
		}
		for t := 0; t < nk; t++ {
			for h := 0; h < heads; h++ {
				s[t*heads+h] = float32(float64(s[t*heads+h]) / z[h])
			}
		}
		o := out[i*os : i*os+n]
		clear(o)
		for t := 0; t < nk; t++ {
			simdk.AxpyHeads(o, v[t*ks:t*ks+n], s[t*heads:(t+1)*heads], heads, dh, simd)
		}
	}
}

// evalGNN is GraphNet.forward in eval mode: st's state and opts -> the option
// logits (written into scores) and the value logit.
func (g *Net) evalGNN(gsc *gnScratch, st *policynet.State, opts []policynet.Option, scores []float32) float32 {
	n := g.gnn
	sc := &gsc.gnn
	d := n.d
	nc := len(st.Cards)
	et := func(t int) []float32 { return n.edgeType[t*d : (t+1)*d] }
	nt := func(t int) []float32 { return n.nodeType[t*d : (t+1)*d] }

	// ---- leaves: a card's children are one leaf per identity row and its raw leaf
	nl := nc
	for i := range st.Cards {
		nl += len(st.Cards[i].Rows)
	}
	leaves := takeF(&sc.leaves, nl*d)
	leafSeg := takeI(&sc.leafSeg, nl)
	// every card's raw leaf in one GEMM (Raw zero-padded or truncated to rawW)
	rw := n.rawW
	raws := takeF(&sc.raws, nc*rw)
	for i := range st.Cards {
		r := raws[i*rw : (i+1)*rw]
		clear(r[copy(r, st.Cards[i].Raw):])
	}
	rl := takeF(&sc.rl, nc*d)
	n.rawLeaf.gemm(rl, raws, nc)
	li := 0
	for i := range st.Cards {
		c := &st.Cards[i]
		for _, f := range c.Rows {
			leaf(leaves[li*d:(li+1)*d], n.ltab, int(f.Row), f.Value, et(gnEdgeIdent))
			leafSeg[li] = i
			li++
		}
		dst := leaves[li*d : (li+1)*d]
		copy(dst, rl[i*d:(i+1)*d])
		addRow(dst, et(gnEdgeRaw))
		leafSeg[li] = i
		li++
	}
	// the root's leaves: DENSE, then one per sparse row, after 3 node kids
	nk := 3 + 1 + len(st.Sparse)
	kids := takeF(&sc.kids, nk*d)
	dl := kids[3*d : 4*d]
	n.denseLeaf.gemm(dl, fit(&sc.fit, st.Dense, policynet.DenseWidth), 1)
	addRow(dl, et(gnEdgeDense))
	for i, f := range st.Sparse {
		leaf(kids[(4+i)*d:(5+i)*d], n.ltab, int(f.Row), f.Value, et(gnEdgeSparse))
	}
	kidSeg := takeI(&sc.kidSeg, max(nk, nc)) // the zone layer's segments, then the root's (all 0)

	// ---- typed initial nodes
	card := takeF(&sc.card, nc*d)
	for i := 0; i < nc; i++ {
		copy(card[i*d:(i+1)*d], nt(gnNodeCard))
	}
	zone := takeF(&sc.zone, 4*d)
	copy(zone, n.nodeType[gnNodeMyBF*d:(gnNodeStack+1)*d])
	play := takeF(&sc.play, 2*d)
	copy(play, n.nodeType[gnNodeMe*d:(gnNodeOpp+1)*d])
	root := takeF(&sc.root, d)
	copy(root, nt(gnNodeRoot))

	// zone children: every card whose group is a zone, plus the edge type
	zc := takeF(&sc.zc, nc*d)
	for p := range n.local {
		lay := &n.local[p]
		localLayer(n, &lay[0], sc, card, leaves, leafSeg, nc, nl)
		ne := 0
		zoneSeg := kidSeg[:0]
		for i := range st.Cards {
			gr := int(st.Cards[i].Group)
			if gr >= 4 {
				continue
			}
			copy(zc[ne*d:(ne+1)*d], card[i*d:(i+1)*d])
			addRow(zc[ne*d:(ne+1)*d], et(gnEdgeCardZone))
			zoneSeg = append(zoneSeg, gr)
			ne++
		}
		localLayer(n, &lay[1], sc, zone, zc, zoneSeg, 4, ne)
		// player children: zones 0, 2 under me, zone 1 under the opponent
		pc := kids[:3*d] // the root's node kids are rebuilt after this layer
		for i, z := range gnPlayerZones {
			copy(pc[i*d:(i+1)*d], zone[z*d:(z+1)*d])
			addRow(pc[i*d:(i+1)*d], et(gnEdgeZonePlayer))
		}
		localLayer(n, &lay[2], sc, play, pc, gnPlayerOfZones[:], 2, 3)
		// root children: the two players, the stack zone, DENSE, the sparse rows
		for i := 0; i < 2; i++ {
			copy(kids[i*d:(i+1)*d], play[i*d:(i+1)*d])
			addRow(kids[i*d:(i+1)*d], et(gnEdgePlayerRoot))
		}
		copy(kids[2*d:3*d], zone[3*d:4*d])
		addRow(kids[2*d:3*d], et(gnEdgeStackRoot))
		rseg := kidSeg[:nk]
		clear(rseg)
		localLayer(n, &lay[3], sc, root, kids, rseg, 1, nk)
	}

	// ---- global tokens [root, me, opp, 4 zones, cards]
	T := gnFixed + nc
	x := takeF(&sc.x, T*d)
	copy(x[0:d], root)
	copy(x[d:3*d], play)
	copy(x[3*d:7*d], zone)
	copy(x[gnFixed*d:], card)
	globalPass(n, sc, x, T)

	v1 := takeF(&sc.v1, 1)
	gnnHead(n.value, x[:d], 1, v1, sc)
	value := v1[0]
	if len(opts) > 0 {
		gnnOptions(n, sc, x, T, opts, scores)
	}
	return value
}

// gnnHead runs a head (Linear, ReLU, ..., Linear to 1) on n rows of x,
// writing each row's logit to out.
func gnnHead(layers []gnLinear, x []float32, n int, out []float32, sc *gnnScratch) {
	cur := x
	a, b := &sc.ha1, &sc.hb1
	for i := range layers {
		l := &layers[i]
		y := takeF(a, n*l.out)
		l.gemm(y, cur, n)
		if i == len(layers)-1 {
			copy(out[:n], y) // the last layer has one output
			return
		}
		relu(y)
		cur = y
		a, b = b, a
	}
}

// gnnOptions writes the logit of each option (_Net._option_scores).
func gnnOptions(n *gnGNN, sc *gnnScratch, x []float32, T int, opts []policynet.Option, scores []float32) {
	d, heads, dh := n.d, n.heads, n.d/n.heads
	nc, no := T-gnFixed, len(opts)
	o := takeF(&sc.o, no*d)
	ha := takeF(&sc.ha, no*d)
	hb := takeF(&sc.hb, no*d)
	// o = opt_in.dense(dense) + slots + hashed bag + bot, every option's dense row in one GEMM
	const ow = policynet.OptionDenseWidth
	od := takeF(&sc.od, no*ow)
	for j := range opts {
		r := od[j*ow : (j+1)*ow]
		clear(r[copy(r, opts[j].Dense):])
	}
	n.odense.gemm(o, od, no)
	for j := range opts {
		op := &opts[j]
		dst := o[j*d : (j+1)*d]
		for _, f := range op.Slots {
			if r := int(f.Row); r < gnSlotRow {
				axpy(dst, n.slots[r*d:(r+1)*d], f.Value)
			}
		}
		for _, f := range op.Hashed {
			if r := int(f.Row); r < gnRows {
				axpy(dst, n.htab[r*d:(r+1)*d], f.Value)
			}
		}
		if op.BotPick {
			addRow(dst, n.bot)
		}
		// h_ea/h_eb: the referenced cards' final tokens, or none_a/none_b
		a, b := ha[j*d:(j+1)*d], hb[j*d:(j+1)*d]
		if k := int(op.EntA) - 1; k >= 0 && k < nc {
			copy(a, x[(gnFixed+k)*d:(gnFixed+k+1)*d])
		} else {
			copy(a, n.noneA)
		}
		if k := int(op.EntB) - 1; k >= 0 && k < nc {
			copy(b, x[(gnFixed+k)*d:(gnFixed+k+1)*d])
		} else {
			copy(b, n.noneB)
		}
	}
	// + ea_proj(h_ea) + eb_proj(h_eb)
	ta := takeF(&sc.ta, no*d)
	n.eaProj.gemm(ta, ha, no)
	addRow(o, ta)
	n.ebProj.gemm(ta, hb, no)
	addRow(o, ta)

	// the cross block: q = Q(nq(o)) over K, V = KV(nkv(x)); h = o + out(attn); h += FFN(n2(h))
	ln := takeF(&sc.ln, max(T, no)*d)
	n.crossNKV.rows(ln, x, T, d)
	kv := takeF(&sc.kv, T*2*d)
	n.crossKV.gemm(kv, ln, T)
	n.crossNQ.rows(ln, o, no, d)
	q := takeF(&sc.q, no*d)
	n.crossQ.gemm(q, ln, no)
	q2 := takeF(&sc.q2, no*d)
	attend(q, d, kv, kv[d:], 2*d, no, T, heads, dh, 1/math.Sqrt(float64(dh)), q2, d, sc)
	h := takeF(&sc.out, no*d)
	n.crossOut.gemm(h, q2, no)
	addRow(h, o)
	ffn(sc, &n.crossN2, &n.crossFF1, &n.crossFF2, h, no, d)

	// score = MLP([root ‖ h ‖ h_ea ‖ h_eb]), every option in one batch
	z := takeF(&sc.z, no*4*d)
	for j := 0; j < no; j++ {
		zj := z[j*4*d : (j+1)*4*d]
		copy(zj[:d], x[:d])
		copy(zj[d:2*d], h[j*d:(j+1)*d])
		copy(zj[2*d:3*d], ha[j*d:(j+1)*d])
		copy(zj[3*d:], hb[j*d:(j+1)*d])
	}
	gnnHead(n.policy, z, no, scores, sc)
}
