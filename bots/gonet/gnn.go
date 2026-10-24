package gonet

import (
	"fmt"
	"math"

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
// Numerics: attention logits and softmax run in float64 (the reference uses
// fp32 with a log-sum-exp); every loop has a fixed summation order, so a
// forward is deterministic.

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
// rows alone (a view, no copy): a child contributes keys and values only.
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
	noneAP, noneBP []float32 // ea_proj(none_a), eb_proj(none_b)

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
	score, mx, sum                       []float64
	o, ha, hb, q, q2, z, ha1, hb1        []float32
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
			l[i] = gnLocalLayer{n1: norm(pre+".n1", d), n2: norm(pre+".n2", d), qkv: qkv,
				kv:  gnLinear{in: d, out: 2 * d, w: qkv.w[d*d:], b: qkv.b[d:]},
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
	g.noneAP, g.noneBP = make([]float32, d), make([]float32, d)
	g.eaProj.apply(g.noneAP, g.noneA)
	g.ebProj.apply(g.noneBP, g.noneB)
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

// geluExact is F.gelu's default (erf) form.
func geluExact(x float32) float32 {
	v := float64(x)
	return float32(0.5 * v * (1 + math.Erf(v/math.Sqrt2)))
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
	for r := 0; r < n; r++ {
		n2.apply(ln[r*d:(r+1)*d], x[r*d:(r+1)*d])
	}
	hid := takeF(&sc.hid, n*fc1.out)
	fc1.applyN(hid, ln, n, true)
	for i, v := range hid {
		hid[i] = geluExact(v)
	}
	tmp := takeF(&sc.tmp, n*d)
	fc2.applyN(tmp, hid, n, true)
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
	for s := 0; s < S; s++ {
		l.n1.apply(heavy[s*d:(s+1)*d], node[s*d:(s+1)*d])
	}
	qkv := takeF(&sc.qkv, S*3*d)
	l.qkv.applyN(qkv, heavy, S, true)
	for e := 0; e < E; e++ {
		l.n1.apply(heavy[e*d:(e+1)*d], child[e*d:(e+1)*d])
	}
	kv := takeF(&sc.kv, E*2*d)
	l.kv.applyN(kv, heavy, E, true)

	// logits: row r < S is node r's own key; row S+e is child e's
	total := S + E
	segAll := takeI(&sc.segAll, total)
	for i := 0; i < S; i++ {
		segAll[i] = i
	}
	copy(segAll[S:], seg[:E])
	score := takeD(&sc.score, total*heads)
	inv := 1 / math.Sqrt(float64(dh))
	for r := 0; r < total; r++ {
		p := segAll[r]
		q := qkv[p*3*d : p*3*d+d]
		var k []float32
		if r < S {
			k = qkv[r*3*d+d : r*3*d+2*d]
		} else {
			k = kv[(r-S)*2*d : (r-S)*2*d+d]
		}
		for h := 0; h < heads; h++ {
			var acc float64
			for j := h * dh; j < (h+1)*dh; j++ {
				acc += float64(q[j]) * float64(k[j])
			}
			score[r*heads+h] = acc * inv
		}
	}
	// segment softmax: every segment holds its own node, so none is empty
	mx, sum := takeD(&sc.mx, S*heads), takeD(&sc.sum, S*heads)
	for i := range mx {
		mx[i] = math.Inf(-1)
		sum[i] = 0
	}
	for r := 0; r < total; r++ {
		b := segAll[r] * heads
		for h := 0; h < heads; h++ {
			mx[b+h] = math.Max(mx[b+h], score[r*heads+h])
		}
	}
	for r := 0; r < total; r++ {
		b := segAll[r] * heads
		for h := 0; h < heads; h++ {
			v := math.Exp(score[r*heads+h] - mx[b+h])
			score[r*heads+h] = v
			sum[b+h] += v
		}
	}
	out := takeF(&sc.out, S*d)
	acc := takeD(&sc.mx, S*d) // mx is spent; reuse it as the float64 accumulator
	clear(acc)
	for r := 0; r < total; r++ {
		p := segAll[r]
		var v []float32
		if r < S {
			v = qkv[r*3*d+2*d : (r+1)*3*d]
		} else {
			v = kv[(r-S)*2*d+d : (r-S+1)*2*d]
		}
		for h := 0; h < heads; h++ {
			a := score[r*heads+h] / sum[p*heads+h]
			o := acc[p*d+h*dh : p*d+(h+1)*dh]
			for j := range o {
				o[j] += a * float64(v[h*dh+j])
			}
		}
	}
	for i := range out {
		out[i] = float32(acc[i])
	}
	tmp := takeF(&sc.tmp, S*d)
	l.out.applyN(tmp, out, S, true)
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
		for r := 0; r < T; r++ {
			b.n1.apply(ln[r*d:(r+1)*d], x[r*d:(r+1)*d])
		}
		qkv := takeF(&sc.qkv, T*3*d)
		b.qkv.applyN(qkv, ln, T, true)
		out := takeF(&sc.out, T*d)
		attend(qkv, 3*d, qkv[d:], qkv[2*d:], 3*d, T, T, heads, dh, inv, out, d, sc)
		tmp := takeF(&sc.tmp, T*d)
		b.out.applyN(tmp, out, T, true)
		addRow(x[:T*d], tmp)
		ffn(sc, &b.n2, &b.fc1, &b.fc2, x, T, d)
	}
	for r := 0; r < T; r++ {
		g.norm.apply(x[r*d:(r+1)*d], x[r*d:(r+1)*d])
	}
}

// attend is softmax(q·kᵀ·inv)·v per head for nq queries over nk keys. Row i
// of q starts at q[i*qs], row t of k and v at k[t*ks], v[t*ks]; the result's
// row i is out[i*os : i*os+heads*dh].
func attend(q []float32, qs int, k, v []float32, ks, nq, nk, heads, dh int, inv float64, out []float32, os int, sc *gnnScratch) {
	p := takeD(&sc.score, nk)
	acc := takeD(&sc.sum, dh)
	for i := 0; i < nq; i++ {
		for h := 0; h < heads; h++ {
			qi := q[i*qs+h*dh : i*qs+(h+1)*dh]
			m := math.Inf(-1)
			for t := 0; t < nk; t++ {
				kt := k[t*ks+h*dh : t*ks+(h+1)*dh]
				var s float64
				for j := range qi {
					s += float64(qi[j]) * float64(kt[j])
				}
				s *= inv
				p[t] = s
				m = math.Max(m, s)
			}
			var z float64
			for t := 0; t < nk; t++ {
				p[t] = math.Exp(p[t] - m)
				z += p[t]
			}
			clear(acc)
			for t := 0; t < nk; t++ {
				a := p[t] / z
				vt := v[t*ks+h*dh : t*ks+(h+1)*dh]
				for j := range acc {
					acc[j] += a * float64(vt[j])
				}
			}
			o := out[i*os+h*dh : i*os+(h+1)*dh]
			for j := range o {
				o[j] = float32(acc[j])
			}
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
	li := 0
	for i := range st.Cards {
		c := &st.Cards[i]
		for _, f := range c.Rows {
			leaf(leaves[li*d:(li+1)*d], n.ltab, int(f.Row), f.Value, et(gnEdgeIdent))
			leafSeg[li] = i
			li++
		}
		dst := leaves[li*d : (li+1)*d]
		n.rawLeaf.apply(dst, fit(&sc.fit, c.Raw, n.rawW))
		addRow(dst, et(gnEdgeRaw))
		leafSeg[li] = i
		li++
	}
	// the root's leaves: DENSE, then one per sparse row, after 3 node kids
	nk := 3 + 1 + len(st.Sparse)
	kids := takeF(&sc.kids, nk*d)
	dl := kids[3*d : 4*d]
	n.denseLeaf.apply(dl, fit(&sc.fit, st.Dense, policynet.DenseWidth))
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

	value := gnnHead(n.value, x[:d], sc)
	if len(opts) > 0 {
		gnnOptions(n, sc, x, T, opts, scores)
	}
	return value
}

// gnnHead runs a head (Linear, ReLU, ..., Linear to 1) on one row.
func gnnHead(layers []gnLinear, x []float32, sc *gnnScratch) float32 {
	cur := x
	a, b := &sc.ha1, &sc.hb1
	for i := range layers {
		l := &layers[i]
		if i == len(layers)-1 {
			return l.b[0] + dot(l.w[:l.in], cur)
		}
		y := takeF(a, l.out)
		l.apply(y, cur)
		relu(y)
		cur = y
		a, b = b, a
	}
	return 0
}

// gnnOptions writes the logit of each option (_Net._option_scores).
func gnnOptions(n *gnGNN, sc *gnnScratch, x []float32, T int, opts []policynet.Option, scores []float32) {
	d, heads, dh := n.d, n.heads, n.d/n.heads
	nc, no := T-gnFixed, len(opts)
	o := takeF(&sc.o, no*d)
	ha := takeF(&sc.ha, no*d)
	hb := takeF(&sc.hb, no*d)
	var odense [policynet.OptionDenseWidth]float32
	tmp := takeF(&sc.tmp, d)
	for j := range opts {
		op := &opts[j]
		dst := o[j*d : (j+1)*d]
		clear(odense[:])
		copy(odense[:], op.Dense)
		n.odense.apply(dst, odense[:])
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
			n.eaProj.apply(tmp, a)
			addRow(dst, tmp)
		} else {
			copy(a, n.noneA)
			addRow(dst, n.noneAP)
		}
		if k := int(op.EntB) - 1; k >= 0 && k < nc {
			copy(b, x[(gnFixed+k)*d:(gnFixed+k+1)*d])
			n.ebProj.apply(tmp, b)
			addRow(dst, tmp)
		} else {
			copy(b, n.noneB)
			addRow(dst, n.noneBP)
		}
	}

	// the cross block: q = Q(nq(o)) over K, V = KV(nkv(x)); h = o + out(attn); h += FFN(n2(h))
	ln := takeF(&sc.ln, max(T, no)*d)
	for t := 0; t < T; t++ {
		n.crossNKV.apply(ln[t*d:(t+1)*d], x[t*d:(t+1)*d])
	}
	kv := takeF(&sc.kv, T*2*d)
	n.crossKV.applyN(kv, ln, T, true)
	for j := 0; j < no; j++ {
		n.crossNQ.apply(ln[j*d:(j+1)*d], o[j*d:(j+1)*d])
	}
	q := takeF(&sc.q, no*d)
	n.crossQ.applyN(q, ln, no, true)
	q2 := takeF(&sc.q2, no*d)
	attend(q, d, kv, kv[d:], 2*d, no, T, heads, dh, 1/math.Sqrt(float64(dh)), q2, d, sc)
	h := takeF(&sc.out, no*d)
	n.crossOut.applyN(h, q2, no, true)
	addRow(h, o)
	ffn(sc, &n.crossN2, &n.crossFF1, &n.crossFF2, h, no, d)

	// score = MLP([root ‖ h ‖ h_ea ‖ h_eb])
	z := takeF(&sc.z, 4*d)
	for j := 0; j < no; j++ {
		copy(z[:d], x[:d])
		copy(z[d:2*d], h[j*d:(j+1)*d])
		copy(z[2*d:3*d], ha[j*d:(j+1)*d])
		copy(z[3*d:], hb[j*d:(j+1)*d])
		scores[j] = gnnHead(n.policy, z, sc)
	}
}
