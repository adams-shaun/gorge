package netsimd

import (
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"unsafe"

	"github.com/adams-shaun/gorge/bots/policynet"
)

const (
	magic     = "DZGW0001"
	align     = 64
	groups    = 4
	poolW     = 2 * groups
	tableRows = policynet.TableRows
	slotRows  = policynet.OptionSlotWidth
	rawW      = policynet.EntityRawWidth
	denseW    = policynet.DenseWidth
	optDense  = policynet.OptionDenseWidth
)

// lin is one Linear layer, transposed and padded: wt[i*outPad+o] = W[o][i]
// for i < in, o < out, zero beyond out; b is outPad wide (zeros without a
// bias). gemvT walks it row by input, accumulating all outputs at once.
type lin struct {
	in, out, outPad int
	wt, b           []float32
}

type norm struct {
	w, b []float32
	eps  float32
}

// Net is the loaded network. Shape: d is the state/option width, ff the
// block width, e the embedding width, cd the card width and hh the head
// hidden width.
//
// The AVX-512 kernels are 64-lane (ln64/dot64/addv64), so a net with
// d == hh == 64 lands on the assembly fast path and every other shape runs
// the width-generic Go kernels (lnN/dotN/addvN). ff must still be a multiple
// of 32 (gemvT pads the output to 32). A wide net is slower per lane but the
// same code path; see fast.
type Net struct {
	d, ff, e, cd, hh int
	rawW             int  // entity raw width the card_raw layer reads
	fast             bool // d == hh == 64: the 64-lane assembly applies

	table               []float32 // rows×e
	cardRaw, cardRows   lin       // rawW→cd, e→cd
	cardPhi             lin       // cd→cd
	denseIn, sparseProj lin       // denseW→d, e→d
	poolProj            lin       // poolW·cd→d
	bnorm               norm
	fc1, fc2            lin // d→ff, ff→d
	fnorm               norm
	slots               []float32 // slotRows×d
	htab                []float32 // rows×d: hproj(table[r])
	odense              lin       // optDense→d
	bot                 []float32 // d
	eaProj, ebProj      lin       // cd→d
	noneA, noneB        []float32 // d: ea_proj(none_a), eb_proj(none_b)
	onorm               norm
	pol0s, pol0o        lin       // d→hh (pol0s carries policy.0's bias)
	pol2                lin       // hh→hh
	pol4w               []float32 // hh
	pol4b               float32
	val0                lin // d→hh
	val2w               []float32
	val2b               float32

	cache *cardCache
	zero  []float32 // d zeros (an accumulator for a bias-free gemv)
}

// Scratch is one evaluation's working storage; grown on first use, then
// reused with no allocation. One per goroutine.
type Scratch struct {
	ent        []float32 // per card: cd
	raw, u     []float32 // rawW, cd
	bag32      []float32 // e
	pool       []float32 // poolW·cd
	dense      []float32 // denseW
	s, t, hs   []float32 // d
	h          []float32 // ff
	vh         []float32 // hh
	o, oh, oh2 []float32 // d, hh, hh
	od         []float32 // optDense
	pa, pb     []float32 // per card: d
	paOK, pbOK []bool
	keys       []uint64
	hbuf       []byte
	groups     []uint8   // per card: its pooling group
	ent1       []float32 // CardVector's output
}

// aligned returns n float32s starting on a 64-byte boundary.
func aligned(n int) []float32 {
	buf := make([]float32, n+16)
	off := (align - int(uintptr(unsafe.Pointer(&buf[0]))&(align-1))) / 4 % 16
	return buf[off : off+n : off+n]
}

func growF(s []float32, n int) []float32 {
	if cap(s) < n {
		return aligned(n + n/4)[:n]
	}
	return s[:n]
}

func growG(s []uint8, n int) []uint8 {
	if cap(s) < n {
		return make([]uint8, n, n+n/4)
	}
	return s[:n]
}

func growB(s []bool, n int) []bool {
	if cap(s) < n {
		return make([]bool, n, n+n/4)
	}
	s = s[:n]
	clear(s)
	return s
}

func roundUp(n, m int) int { return (n + m - 1) / m * m }

type header struct {
	Format  string             `json:"format"`
	Arch    string             `json:"arch"`
	Config  map[string]float64 `json:"config"`
	Consts  map[string]int     `json:"consts"`
	Eps     map[string]float64 `json:"layernorm_eps"`
	Tensors []struct {
		Name   string `json:"name"`
		Shape  []int  `json:"shape"`
		Offset int64  `json:"offset"`
		NBytes int64  `json:"nbytes"`
	} `json:"tensors"`
}

// Load reads a .dzgw file (the same format bots/gonet.Load reads) into the
// transposed, padded layout. With cardCache > 0 the card vectors are memoised
// across evaluations by the card's encoding, as gonet's are.
func Load(path string, cardCache int) (*Net, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if len(raw) < 16 || string(raw[:8]) != magic {
		return nil, fmt.Errorf("netsimd %s: not a .dzgw file", path)
	}
	hl := int64(binary.LittleEndian.Uint64(raw[8:16]))
	if hl <= 0 || hl > int64(len(raw))-16 {
		return nil, fmt.Errorf("netsimd %s: bad header length %d", path, hl)
	}
	var h header
	if err := json.Unmarshal(raw[16:16+hl], &h); err != nil {
		return nil, fmt.Errorf("netsimd %s: header: %w", path, err)
	}
	if h.Format != "dzgw-1" || h.Arch != "mlp" {
		return nil, fmt.Errorf("netsimd %s: format %q arch %q, want dzgw-1 mlp", path, h.Format, h.Arch)
	}
	rawIn := h.Consts["RAW_W"]
	if rawIn != policynet.EntityRawWidth && rawIn != policynet.EntityRawWidthV1 && rawIn != policynet.EntityRawWidthV3 {
		return nil, fmt.Errorf("netsimd %s: RAW_W = %d, this build reads %d, %d or %d", path, rawIn, policynet.EntityRawWidthV1, policynet.EntityRawWidthV3, policynet.EntityRawWidth)
	}
	want := map[string]int{"DENSE_W": denseW, "OPT_DENSE_W": optDense, "TABLE_ROWS": tableRows, "SLOT_ROWS": slotRows, "GROUPS": groups}
	for k, v := range want {
		if h.Consts[k] != v {
			return nil, fmt.Errorf("netsimd %s: %s = %d, this build has %d", path, k, h.Consts[k], v)
		}
	}
	start := (16 + hl + align - 1) / align * align
	tensors := map[string][]float32{}
	shapes := map[string][]int{}
	for _, t := range h.Tensors {
		n := int64(1)
		for _, d := range t.Shape {
			if d < 0 || d > 1<<30 {
				return nil, fmt.Errorf("netsimd %s: tensor %s: bad shape %v", path, t.Name, t.Shape)
			}
			n *= int64(d)
			if n > 1<<31 {
				return nil, fmt.Errorf("netsimd %s: tensor %s: shape %v is too large", path, t.Name, t.Shape)
			}
		}
		if t.Offset < 0 || t.NBytes != 4*n || t.Offset > int64(len(raw))-start-t.NBytes {
			return nil, fmt.Errorf("netsimd %s: tensor %s: bad extent", path, t.Name)
		}
		b := raw[start+t.Offset : start+t.Offset+t.NBytes]
		f := aligned(int(n))
		for i := range f {
			f[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[4*i:]))
		}
		tensors[t.Name], shapes[t.Name] = f, t.Shape
	}
	cfg := func(k string) int { return int(h.Config[k]) }
	g := &Net{d: cfg("d"), ff: cfg("ff"), e: cfg("d_emb"), cd: cfg("card_d"), hh: cfg("head_hidden"), rawW: rawIn}
	if g.d <= 0 || g.hh <= 0 || g.e <= 0 || g.cd <= 0 || g.ff%32 != 0 {
		return nil, fmt.Errorf("netsimd %s: shape d=%d ff=%d e=%d cd=%d hh=%d: need d,hh,e,cd > 0 and ff%%32==0", path, g.d, g.ff, g.e, g.cd, g.hh)
	}
	g.fast = g.d == 64 && g.hh == 64
	var errs []error
	get := func(name string, shape ...int) []float32 {
		t, ok := tensors[name]
		if !ok {
			errs = append(errs, fmt.Errorf("missing tensor %s", name))
			return nil
		}
		s := shapes[name]
		ok = len(s) == len(shape)
		for i := range s {
			ok = ok && s[i] == shape[i]
		}
		if !ok {
			errs = append(errs, fmt.Errorf("tensor %s: shape %v, want %v", name, s, shape))
			return nil
		}
		return t
	}
	// transpose W (out×in) into wt (in×outPad)
	mk := func(w []float32, in, out int, b []float32) lin {
		l := lin{in: in, out: out, outPad: roundUp(out, 32)}
		l.wt = aligned(in * l.outPad)
		l.b = aligned(l.outPad)
		if w == nil {
			return l
		}
		for o := 0; o < out; o++ {
			for i := 0; i < in; i++ {
				l.wt[i*l.outPad+o] = w[o*in+i]
			}
		}
		copy(l.b, b)
		return l
	}
	linear := func(name string, in, out int, bias bool) lin {
		w := get(name+".weight", out, in)
		var b []float32
		if bias {
			b = get(name+".bias", out)
		}
		return mk(w, in, out, b)
	}
	nrm := func(name string, d int) norm {
		eps, ok := h.Eps[name]
		if !ok {
			errs = append(errs, fmt.Errorf("no layernorm_eps for %s", name))
		}
		return norm{w: get(name+".weight", d), b: get(name+".bias", d), eps: float32(eps)}
	}
	d, e, cd, hh := g.d, g.e, g.cd, g.hh
	g.table = get("table.weight", tableRows, e)
	g.cardRaw = linear("card_raw", rawIn, cd, true)
	g.cardRows = linear("card_rows", e, cd, false)
	g.cardPhi = linear("card_phi", cd, cd, true)
	g.denseIn = linear("dense_in", denseW, d, true)
	g.sparseProj = linear("sparse_proj", e, d, false)
	g.poolProj = linear("pool_proj", poolW*cd, d, true)
	g.bnorm = nrm("blocks.0.norm", d)
	g.fc1 = linear("blocks.0.fc1", d, g.ff, true)
	g.fc2 = linear("blocks.0.fc2", g.ff, d, true)
	g.fnorm = nrm("norm", d)
	g.slots = get("opt_in.slots.weight", slotRows, d)
	hproj := linear("opt_in.hproj", e, d, false)
	g.odense = linear("opt_in.dense", optDense, d, true)
	g.bot = get("opt_in.bot", d)
	g.eaProj = linear("ea_proj", cd, d, true)
	g.ebProj = linear("eb_proj", cd, d, true)
	noneA, noneB := get("none_a", cd), get("none_b", cd)
	g.onorm = nrm("opt_norm", d)
	if cfg("policy_layers") != 2 || cfg("value_layers") != 1 {
		errs = append(errs, fmt.Errorf("policy_layers %d value_layers %d: this build reads 2 and 1", cfg("policy_layers"), cfg("value_layers")))
	}
	if cfg("layers") != 1 {
		errs = append(errs, fmt.Errorf("layers %d: this build reads one trunk block", cfg("layers")))
	}
	p0w := get("policy.0.weight", hh, 2*d)
	p0b := get("policy.0.bias", hh)
	g.pol2 = linear("policy.2", hh, hh, true)
	g.pol4w = get("policy.4.weight", 1, hh)
	g.val0 = linear("value.0", d, hh, true)
	g.val2w = get("value.2.weight", 1, hh)
	if b := get("policy.4.bias", 1); b != nil {
		g.pol4b = b[0]
	}
	if b := get("value.2.bias", 1); b != nil {
		g.val2b = b[0]
	}
	if len(errs) > 0 {
		return nil, fmt.Errorf("netsimd %s: %v", path, errs)
	}
	// policy.0 on [s ‖ o] = W[:, :d]·s + b + W[:, d:]·o
	ws, wo := make([]float32, hh*d), make([]float32, hh*d)
	for o := 0; o < hh; o++ {
		copy(ws[o*d:(o+1)*d], p0w[o*2*d:o*2*d+d])
		copy(wo[o*d:(o+1)*d], p0w[o*2*d+d:(o+1)*2*d])
	}
	g.pol0s = mk(ws, d, hh, p0b)
	g.pol0o = mk(wo, d, hh, nil)
	// hproj(table[r]) for every row, and the none projections
	g.htab = aligned(tableRows * d)
	zero := aligned(d)
	g.zero = zero
	for r := 0; r < tableRows; r++ {
		gemvT(&hproj.wt[0], &g.table[r*e], &zero[0], &g.htab[r*d], e, d, false)
	}
	g.noneA, g.noneB = aligned(d), aligned(d)
	gemvT(&g.eaProj.wt[0], &noneA[0], &g.eaProj.b[0], &g.noneA[0], cd, d, false)
	gemvT(&g.ebProj.wt[0], &noneB[0], &g.ebProj.b[0], &g.noneB[0], cd, d, false)
	if cardCache > 0 {
		g.cache = newCardCache(cardCache, cd)
	}
	return g, nil
}

// RawWidth is the entity raw width the network's card_raw layer reads: 68
// for a format-1 network, policynet.EntityRawWidth for the current encoding.
func (g *Net) RawWidth() int { return g.rawW }

// Eval runs the network on st, writes each option's logit into scores
// (len(opts)) and returns the value logit — gonet.Net.Eval's contract.
func (g *Net) Eval(sc *Scratch, st *policynet.State, opts []policynet.Option, scores []float32) float32 {
	cd := g.cd
	nc := len(st.Cards)

	// card vectors
	sc.ent = growF(sc.ent, max(nc, 1)*cd)
	sc.groups = growG(sc.groups, nc)
	if g.cache != nil {
		if cap(sc.keys) < nc {
			sc.keys = make([]uint64, nc, nc+nc/4)
		}
		sc.keys = sc.keys[:nc]
	}
	for k := range st.Cards {
		c := &st.Cards[k]
		ent := &sc.ent[k*cd]
		sc.groups[k] = c.Group
		if g.cache != nil {
			sc.keys[k] = hashCard(&sc.hbuf, c)
			if g.cache.get(sc.keys[k], ent) {
				continue
			}
		}
		g.cardFull(sc, c.Raw, c.Rows, ent)
		if g.cache != nil {
			g.cache.put(sc.keys[k], ent)
		}
	}
	return g.trunk(sc, nc, st.Dense, st.Sparse, opts, scores)
}

// cardFull writes the card vector of (raw, rows) to ent:
// u = relu(card_raw(raw) + card_rows(bag(rows))); c = relu(card_phi(u)).
func (g *Net) cardFull(sc *Scratch, raw []float32, rows []policynet.Feature, ent *float32) {
	cd, e := g.cd, g.e
	sc.raw = growF(sc.raw, g.rawW)
	sc.u = growF(sc.u, cd)
	sc.bag32 = growF(sc.bag32, e)
	n := copy(sc.raw, raw)
	clear(sc.raw[n:])
	gemvT(&g.cardRaw.wt[0], &sc.raw[0], &g.cardRaw.b[0], &sc.u[0], g.rawW, cd, false)
	clear(sc.bag32)
	if len(rows) > 0 {
		bag(&sc.bag32[0], &g.table[0], (*feat)(unsafe.Pointer(&rows[0])), len(rows), tableRows, e)
	}
	gemvT(&g.cardRows.wt[0], &sc.bag32[0], &sc.u[0], &sc.u[0], e, cd, true)
	gemvT(&g.cardPhi.wt[0], &sc.u[0], &g.cardPhi.b[0], ent, cd, cd, true)
}

// trunk is the network after the card vectors: sc.ent holds nc card
// vectors and sc.groups their groups.
func (g *Net) trunk(sc *Scratch, nc int, dense []float32, sparse []policynet.Feature, opts []policynet.Option, scores []float32) float32 {
	return g.trunkBag(sc, nc, dense, sparse, nil, opts, scores)
}

// trunkBag is trunk with the sparse bag's row sum given (bagIn, e wide)
// instead of computed from sparse when bagIn is non-nil.
func (g *Net) trunkBag(sc *Scratch, nc int, dense []float32, sparse []policynet.Feature, bagIn []float32, opts []policynet.Option, scores []float32) float32 {
	d, cd, e := g.d, g.cd, g.e
	sc.bag32 = growF(sc.bag32, e)

	// pools = [0.25·sum per group ‖ max per group] (an empty group's max is 0:
	// card vectors are post-relu, so max(0, c) = max over the group)
	sc.pool = growF(sc.pool, poolW*cd)
	clear(sc.pool)
	for k := 0; k < nc; k++ {
		if gr := int(sc.groups[k]); gr < groups {
			poolAcc(&sc.pool[gr*cd], &sc.pool[(groups+gr)*cd], &sc.ent[k*cd])
		}
	}
	sc.s = growF(sc.s, d)
	s := &sc.s[0]
	gemvT(&g.poolProj.wt[0], &sc.pool[0], &g.poolProj.b[0], s, poolW*cd, d, false)

	// state trunk: s += dense_in(dense) + sparse_proj(bag(sparse))
	sc.dense = growF(sc.dense, denseW)
	n := copy(sc.dense, dense)
	clear(sc.dense[n:])
	gemvT(&g.denseIn.wt[0], &sc.dense[0], s, s, denseW, d, false)
	g.addv(s, &g.denseIn.b[0])
	if bagIn != nil {
		copy(sc.bag32, bagIn[:e])
	} else {
		clear(sc.bag32)
		if len(sparse) > 0 {
			bag(&sc.bag32[0], &g.table[0], (*feat)(unsafe.Pointer(&sparse[0])), len(sparse), tableRows, e)
		}
	}
	gemvT(&g.sparseProj.wt[0], &sc.bag32[0], s, s, e, d, false)
	// block: s += fc2(relu(fc1(LN(s))))
	sc.t = growF(sc.t, d)
	sc.h = growF(sc.h, g.ff)
	g.ln(&sc.t[0], s, &g.bnorm.w[0], &g.bnorm.b[0], g.bnorm.eps)
	gemvT(&g.fc1.wt[0], &sc.t[0], &g.fc1.b[0], &sc.h[0], d, g.ff, true)
	gemvT(&g.fc2.wt[0], &sc.h[0], s, s, g.ff, d, false)
	g.addv(s, &g.fc2.b[0])
	g.ln(s, s, &g.fnorm.w[0], &g.fnorm.b[0], g.fnorm.eps)

	// value head
	sc.vh = growF(sc.vh, g.hh)
	gemvT(&g.val0.wt[0], s, &g.val0.b[0], &sc.vh[0], d, g.hh, true)
	v := g.val2b + g.dot(&g.val2w[0], &sc.vh[0], g.hh)
	if len(opts) == 0 {
		return v
	}

	// options
	hh := g.hh
	sc.hs = growF(sc.hs, hh)
	gemvT(&g.pol0s.wt[0], s, &g.pol0s.b[0], &sc.hs[0], d, hh, false)
	sc.o = growF(sc.o, d)
	sc.oh = growF(sc.oh, hh)
	sc.oh2 = growF(sc.oh2, hh)
	sc.od = growF(sc.od, optDense)
	sc.pa = growF(sc.pa, max(nc, 1)*d)
	sc.pb = growF(sc.pb, max(nc, 1)*d)
	sc.paOK = growB(sc.paOK, nc)
	sc.pbOK = growB(sc.pbOK, nc)
	o := &sc.o[0]
	for j := range opts {
		op := &opts[j]
		// o = dense(opt dense) + bag(slots) + hproj(bag(hashed)) + bot·bot_vec + ea + eb
		n := copy(sc.od, op.Dense)
		clear(sc.od[n:])
		gemvT(&g.odense.wt[0], &sc.od[0], &g.odense.b[0], o, optDense, d, false)
		if len(op.Slots) > 0 {
			bag(o, &g.slots[0], (*feat)(unsafe.Pointer(&op.Slots[0])), len(op.Slots), slotRows, d)
		}
		if len(op.Hashed) > 0 {
			bag(o, &g.htab[0], (*feat)(unsafe.Pointer(&op.Hashed[0])), len(op.Hashed), tableRows, d)
		}
		if op.BotPick {
			g.addv(o, &g.bot[0])
		}
		if k := int(op.EntA) - 1; k >= 0 && k < nc {
			p := &sc.pa[k*d]
			if !sc.paOK[k] {
				gemvT(&g.eaProj.wt[0], &sc.ent[k*cd], &g.eaProj.b[0], p, cd, d, false)
				sc.paOK[k] = true
			}
			g.addv(o, p)
		} else {
			g.addv(o, &g.noneA[0])
		}
		if k := int(op.EntB) - 1; k >= 0 && k < nc {
			p := &sc.pb[k*d]
			if !sc.pbOK[k] {
				gemvT(&g.ebProj.wt[0], &sc.ent[k*cd], &g.ebProj.b[0], p, cd, d, false)
				sc.pbOK[k] = true
			}
			g.addv(o, p)
		} else {
			g.addv(o, &g.noneB[0])
		}
		g.ln(o, o, &g.onorm.w[0], &g.onorm.b[0], g.onorm.eps)
		// policy head: relu(hs + W_o·o), relu(policy.2), policy.4
		gemvT(&g.pol0o.wt[0], o, &sc.hs[0], &sc.oh[0], d, hh, true)
		gemvT(&g.pol2.wt[0], &sc.oh[0], &g.pol2.b[0], &sc.oh2[0], hh, hh, true)
		scores[j] = g.pol4b + g.dot(&g.pol4w[0], &sc.oh2[0], g.hh)
	}
	return v
}

// ---- the card cache ----------------------------------------------------------

// cardCache memoises card vectors by the card's encoding. One map, no lock:
// a Scratch is per goroutine but the cache is per Net, so a multi-goroutine
// caller wraps Eval (the benchmark is single-threaded; gonet shards 64
// ways under mutexes).
type cardCache struct {
	idx  map[uint64]int32
	slab []float32
	cap  int
	w    int
}

func newCardCache(capacity, w int) *cardCache {
	return &cardCache{idx: make(map[uint64]int32, capacity), cap: capacity, w: w}
}

func (c *cardCache) get(k uint64, dst *float32) bool {
	i, ok := c.idx[k]
	if ok {
		copy(f32s(dst, c.w), c.slab[int(i)*c.w:(int(i)+1)*c.w])
	}
	return ok
}

func (c *cardCache) put(k uint64, v *float32) {
	if _, ok := c.idx[k]; ok {
		return
	}
	if len(c.idx) >= c.cap {
		clear(c.idx)
		c.slab = c.slab[:0]
	}
	c.idx[k] = int32(len(c.idx))
	c.slab = append(c.slab, f32s(v, c.w)...)
}

// hashCard hashes what a card's vector reads: the group, raw and identity
// rows (gonet.hashCard's fields; the mix differs, only spread matters).
func hashCard(buf *[]byte, c *policynet.EntityCard) uint64 {
	b := append((*buf)[:0], c.Group)
	b = binary.LittleEndian.AppendUint32(b, uint32(len(c.Raw)))
	for _, x := range c.Raw {
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(x))
	}
	b = binary.LittleEndian.AppendUint32(b, uint32(len(c.Rows)))
	for _, f := range c.Rows {
		b = binary.LittleEndian.AppendUint16(b, f.Row)
		b = binary.LittleEndian.AppendUint32(b, math.Float32bits(f.Value))
	}
	*buf = b
	h := uint64(0x9E3779B97F4A7C15)
	for len(b) >= 8 {
		h = (h ^ binary.LittleEndian.Uint64(b)) * 0xBF58476D1CE4E5B9
		h ^= h >> 31
		b = b[8:]
	}
	for _, c := range b {
		h = (h ^ uint64(c)) * 0xBF58476D1CE4E5B9
		h ^= h >> 31
	}
	return h
}

// ln applies a layernorm over the net's d lanes: the 64-lane assembly when
// the shape is the fast path, the width-generic kernel otherwise.
func (g *Net) ln(y, x, w, b *float32, eps float32) {
	if g.fast {
		ln64(y, x, w, b, eps)
		return
	}
	lnN(y, x, w, b, eps, g.d)
}

// dot is a dot product over the net's head width (d for the option head's
// input, hh for the two scoring heads).
func (g *Net) dot(a, b *float32, n int) float32 {
	if g.fast && n == 64 {
		return dot64(a, b)
	}
	return dotN(a, b, n)
}

// addv adds a bias vector over the net's d lanes.
func (g *Net) addv(y, x *float32) {
	if g.fast {
		addv64(y, x)
		return
	}
	addvN(y, x, g.d)
}
