// Package neteval is dirac/search's network Evaluator: the dirac/feat
// encoder (policynet FeaturesEntity inputs from a dirac position) feeding
// one of two in-process backends, picked when the weights load:
//
//   - netsimd (LoadWeights, the default): gorge's bots/netsimd SIMD forward
//     pass over a .dzgw network (d == head_hidden == 64 runs its dedicated
//     64-lane kernels, any other width the generic ones), on the parts path
//     (cached per-object row sums and card vectors) unless Materialize;
//   - gonet (LoadWeights, when netsimd refuses the shape; LoadGonet to ask
//     for it): gorge's bots/gonet plain float32 forward pass, which takes
//     any MLP export, on the materialized encoding only and with gonet's
//     own caches off (this package caches).
//
// Value is the value head through gonet's sigmoid; Priors are the option
// logits over the dirac Decision's options, with the bot's answer's
// options marked BotPick as azmcts marks them (heuristic.MarkBotPicks);
// dirac/search composes candidate priors from them (single option, the
// subset log-likelihood for declarations, the sum for the fuller kinds,
// then softmax).
//
// This package may import gorge's bots/netsimd, bots/gonet and
// bots/policynet (the networks and their input types) but never gorge's
// engine, which is why it sits outside dirac's core. One Net per
// goroutine: the encoder and the scratch are per instance; the weights
// (netsimd.Net, gonet.Net) are shared. Zero allocations per evaluation
// once the scratch has grown (TestZeroAllocsPerEval).
package neteval

import (
	"fmt"
	"math"
	"sync"
	"unsafe"

	"github.com/adams-shaun/gorge/bots/gonet"
	"github.com/adams-shaun/gorge/bots/netsimd"
	"github.com/adams-shaun/gorge/bots/policynet"
	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/feat"
)

// Weights are a loaded network, shared by every Net: exactly one backend
// is set.
type Weights struct {
	g    *netsimd.Net // netsimd: any shape netsimd's kernels take (parts or materialized)
	gn   *gonet.Net   // gonet: any other MLP shape (materialized)
	vecW int          // netsimd's bag and card-vector width (32 for d=64 nets; a wide net: its d_emb)
}

// Backend names the weights' backend: netsimd or gonet.
func (w *Weights) Backend() string {
	if w.g != nil {
		return "netsimd"
	}
	return "gonet"
}

// LoadWeights reads a .dzgw network: netsimd when it takes the shape (no
// card cache: netsimd's cache is a map that allocates and is not
// goroutine-safe), else gonet, which takes every MLP export netsimd's
// kernels refuse. A file neither loads (not a .dzgw, a gnn export) reports
// both errors.
func LoadWeights(path string) (*Weights, error) {
	g, err := netsimd.Load(path, 0)
	if err != nil {
		gn, gerr := loadGonet(path)
		if gerr != nil {
			return nil, fmt.Errorf("neteval %s: %w; the gonet fallback: %w", path, err, gerr)
		}
		if raw := gn.RawWidth(); raw > feat.EntityRawWidth {
			return nil, fmt.Errorf("neteval %s: net reads entity width %d, this encoder writes %d", path, raw, feat.EntityRawWidth)
		}
		return &Weights{gn: gn}, nil
	}
	if err := checkWidths(path, g.BagWidth(), g.CardWidth(), g.RawWidth()); err != nil {
		return nil, err
	}
	if x := g.ExtWidth(); x != 0 && x != feat.ExtWidth {
		return nil, fmt.Errorf("neteval %s: net reads EXT_W = %d, this encoder writes %d", path, x, feat.ExtWidth)
	}
	return &Weights{g: g, vecW: g.BagWidth()}, nil
}

// LoadGonet loads path through the gonet fallback even when netsimd's
// kernels take its shape: gonet's arithmetic on the materialized encoding,
// the arithmetic gonet.Net answers with directly. netsimd's parts path sums
// the same inputs in another order (Net.Materialize), so only a
// gonet-backed Net is bit-comparable with gonet itself.
func LoadGonet(path string) (*Weights, error) {
	gn, err := loadGonet(path)
	if err != nil {
		return nil, fmt.Errorf("neteval %s: %w", path, err)
	}
	if raw := gn.RawWidth(); raw > feat.EntityRawWidth {
		return nil, fmt.Errorf("neteval %s: net reads entity width %d, this encoder writes %d", path, raw, feat.EntityRawWidth)
	}
	return &Weights{gn: gn}, nil
}

// checkWidths refuses a net this encoder cannot feed exactly. The bag and
// card vectors share one width (vecW). netsimd zero-fills the raw lanes past
// what the encoder writes, so a net reading wider entities than feat encodes
// would run on silent zeros; a narrower legacy net (68, 84) reads an
// unchanged prefix and stays exact.
func checkWidths(path string, bag, card, raw int) error {
	if bag != card {
		return fmt.Errorf("neteval %s: bag width %d, card width %d: the encoder needs one width", path, bag, card)
	}
	if raw > feat.EntityRawWidth {
		return fmt.Errorf("neteval %s: net reads entity width %d, this encoder writes %d", path, raw, feat.EntityRawWidth)
	}
	return nil
}

// gonetMu serializes loadGonet's swap of gonet's package-level cache knobs.
var gonetMu sync.Mutex

// loadGonet loads path through gonet with its evaluation and card caches
// off: gonet.Load reads the sizes from package variables, its caches sit
// behind a mutex every worker would contend on (and allocate per miss),
// and each Net here keeps its own exact cache in front of the backend.
func loadGonet(path string) (*gonet.Net, error) {
	gonetMu.Lock()
	defer gonetMu.Unlock()
	cs, ccs := gonet.CacheSize, gonet.CardCacheSize
	gonet.CacheSize, gonet.CardCacheSize = 0, 0
	defer func() { gonet.CacheSize, gonet.CardCacheSize = cs, ccs }()
	return gonet.Load(path)
}

// Net is one goroutine's evaluator over shared weights and a feat pool.
type Net struct {
	w     *Weights
	enc   *feat.Encoder
	sc    netsimd.Scratch
	pst   policynet.State
	cards []policynet.EntityCard
	fopts []feat.Option
	popts []policynet.Option
	score []float32

	// the last evaluation: Value after Priors at the same position reuses it
	lastS   *dirac.State
	lastSeq uint32
	lastN   uint16
	lastO0  dirac.Action
	lastV   float32
	haveV   bool

	Evals int // forward passes run
	Hits  int // evaluations answered by the cache

	cache []entry // direct-mapped evaluation cache (nil: off)
	// poolGen is mixed into the content keys: State hashes name CardIDs of
	// the matchup's table, so one content key can stand for two encodings
	// in two tables (SetPool bumps it; the cache needs no clearing)
	poolGen uint64

	// Keys picks the cache key (KeyContent, KeyEncoded); VerifyKeys checks
	// one against the other, counting KeyChecks and KeyConflicts (a
	// content key seen with two different encodings).
	Keys         int
	VerifyKeys   bool
	KeyChecks    int
	KeyConflicts int
	seen         map[uint64]uint64

	// Materialize evaluates the materialized encoding (feat EncodeState:
	// the sorted sparse bag, every entity card) instead of the parts
	// (feat EncodeParts: per-object row sums and card vectors cached by
	// version) -- the same inputs, summed in another order.
	Materialize bool
	parts       *feat.Parts
	vecW        int                       // w.vecW
	bagSum      []float32                 // vecW wide
	ext         []policynet.Feature       // the encoded state's extension block (empty when off)
	ptab        [dirac.MaxSeats][]float32 // per seat: a row sum of vecW per parts slot (feat PartSlots)
	pver        [dirac.MaxSeats][feat.PartSlots]uint32
	sumRows     []policynet.Feature
	pvec        [dirac.MaxSeats][feat.PartSlots][2]uint32 // the version of each slot's card vectors, not castable and castable
	pvv         [dirac.MaxSeats][]float32                 // per seat: 2 card vectors of vecW per parts slot
	cin         []netsimd.CardIn
	raws        [][feat.EntityRawWidth]float32
	miss        []int32
	partSeat    uint8

	check *Check // EnableCheck

	// NoViewBase builds every encoder view in full instead of against the
	// search base (feat Encoder.SetBase).
	NoViewBase bool
}

// The row sums and card vectors are cached per seat and parts slot (feat
// PartObj.Idx), each valid while the slot holds the version it was
// computed for (one version names one content); the row sums sit in one
// table so a bag adds them in one SumRows call.

// cacheOpts bounds the option logits a cache entry holds.
const cacheOpts = 48

// entry is one cached evaluation: the inputs' hash, the value probability
// and the option logits.
type entry struct {
	key    uint64
	n      uint16
	ok     bool
	v      float32
	logits [cacheOpts]float32
}

// CacheSize is the evaluation cache's entries per Net (gonet's state cache:
// an exact memo keyed by a hash of the encoded inputs); 0 turns it off.
var CacheSize = 1 << 13

// New returns a Net over w and pool (one per goroutine). Over gonet
// weights it evaluates the materialized encoding (Materialize is set: gonet
// has no parts path) and skips the parts path's buffers.
func New(w *Weights, pool *feat.Pool) *Net {
	const maxCards = dirac.MaxObj + dirac.MaxStack
	n := &Net{
		w: w, enc: feat.NewEncoder(pool), vecW: w.vecW,
		cards: make([]policynet.EntityCard, maxCards),
		fopts: make([]feat.Option, dirac.MaxOptions),
		popts: make([]policynet.Option, dirac.MaxOptions),
		score: make([]float32, dirac.MaxOptions),
	}
	// grow every buffer to its most once, so no evaluation allocates
	if w.g != nil && w.g.ExtWidth() > 0 {
		n.enc.SetExt(feat.ExtAll)
	}
	n.enc.Reserve()
	if w.g != nil {
		n.bagSum = make([]float32, w.vecW)
		n.cin = make([]netsimd.CardIn, maxCards)
		n.raws = make([][feat.EntityRawWidth]float32, maxCards)
		n.miss = make([]int32, 0, maxCards)
		for s := range n.pvv {
			n.pvv[s] = make([]float32, feat.PartSlots*2*w.vecW)
			n.ptab[s] = make([]float32, feat.PartSlots*w.vecW)
		}
		n.sumRows = make([]policynet.Feature, 0, dirac.MaxObj)
		n.pst = policynet.State{Dense: make([]float32, feat.DenseWidth), Cards: n.cards}
		w.g.Eval(&n.sc, &n.pst, n.popts, n.score)
		for i := range n.cards {
			n.cin[i] = netsimd.CardIn{Raw: n.cards[i].Raw}
		}
		w.g.EvalBagExt(&n.sc, n.cin, n.pst.Dense, n.bagSum[:], nil, n.popts, n.score)
	} else {
		n.Materialize = true
	}
	n.pst = policynet.State{}
	n.cards = n.cards[:0]
	if CacheSize > 0 {
		n.cache = make([]entry, CacheSize)
	}
	return n
}

// Input hashing for the cache: a word-at-a-time multiply-xorshift mix over
// the encoded inputs' memory (exact inputs -> one key; 64-bit keys make a
// collision negligible).

func mixw(h, x uint64) uint64 {
	h ^= x
	h *= 0x9e3779b97f4a7c15
	return h ^ h>>29
}

func hu(h, x uint64) uint64 { return mixw(h, x) }

// hbytes mixes n float32/feature words starting at p (4-byte units).
func hwords(h uint64, p unsafe.Pointer, n4 int) uint64 {
	w := unsafe.Slice((*uint64)(p), n4/2)
	for _, x := range w {
		h = mixw(h, x)
	}
	if n4&1 == 1 {
		h = mixw(h, uint64(*(*uint32)(unsafe.Add(p, (n4-1)*4))))
	}
	return h
}

func hfloats(h uint64, f []float32) uint64 {
	if len(f) == 0 {
		return mixw(h, 0)
	}
	return hwords(mixw(h, uint64(len(f))), unsafe.Pointer(&f[0]), len(f))
}

func hfeat(h uint64, fs []policynet.Feature) uint64 {
	if len(fs) == 0 {
		return mixw(h, 1)
	}
	return hwords(mixw(h, uint64(len(fs))), unsafe.Pointer(&fs[0]), 2*len(fs))
}

func hf(h uint64, f float32) uint64 { return mixw(h, uint64(math.Float32bits(f))) }

// stateHash hashes the encoded state n.pst.
func (n *Net) stateHash() uint64 {
	h := hfloats(0x243f6a8885a308d3, n.pst.Dense)
	h = hfeat(h, n.pst.Sparse)
	if len(n.ext) > 0 {
		h = hfeat(h, n.ext)
	}
	h = hu(h, uint64(len(n.pst.Cards)))
	for i := range n.pst.Cards {
		c := &n.pst.Cards[i]
		h = hu(h, uint64(c.Group))
		h = hfloats(h, c.Raw)
		h = hfeat(h, c.Rows)
	}
	return h
}

// lookup returns the cache entry for key (hit reports a match).
func (n *Net) lookup(key uint64) (*entry, bool) {
	if n.cache == nil {
		return nil, false
	}
	e := &n.cache[key&uint64(len(n.cache)-1)]
	return e, e.ok && e.key == key
}

// HasPrior: the network supplies the prior (search.PriorSource).
func (n *Net) HasPrior() bool { return true }

func sigmoid(x float32) float32 { return float32(1 / (1 + math.Exp(-float64(x)))) }

func (n *Net) same(s *dirac.State, d *dirac.Decision) bool {
	return n.haveV && n.lastS == s && n.lastSeq == s.SeqNo() && n.lastN == d.N && (d.N == 0 || n.lastO0 == d.Options[0])
}

func (n *Net) remember(s *dirac.State, d *dirac.Decision, v float32) {
	n.lastS, n.lastSeq, n.lastN, n.lastV, n.haveV = s, s.SeqNo(), d.N, v, true
	if d.N > 0 {
		n.lastO0 = d.Options[0]
	}
}

// state encodes s at d for d.Player into n.pst (views of the encoder's
// storage; feat's layout matches policynet's).
func (n *Net) state(s *dirac.State, d *dirac.Decision, seat uint8) {
	fs := n.enc.EncodeStateFor(s, d, seat)
	n.pst.Dense = fs.Dense[:]
	n.pst.Sparse = featSlice(fs.Sparse)
	n.ext = featSlice(fs.Ext)
	n.cards = n.cards[:0]
	for i := range fs.Cards {
		c := &fs.Cards[i]
		n.cards = append(n.cards, policynet.EntityCard{Group: c.Group, Raw: c.Raw[:], Rows: featSlice(c.Rows[:c.NRows])})
	}
	n.pst.Cards = n.cards
}

// prepParts encodes s at d for seat as parts (feat EncodeParts) and builds
// the network's inputs from them: the bag as the sum of each object's
// cached row sum (in the parts' order) and the fresh entries' rows, and
// each entity card's cached vector, or its raw features to compute (miss
// lists those).
func (n *Net) prepParts(s *dirac.State, d *dirac.Decision, seat uint8) {
	g := n.w.g
	p := n.enc.EncodeParts(s, d, seat)
	se := dirac.SeatIx(seat) // the per-seat caches (ptab, pver, pvec, pvv) hold every seat
	n.parts, n.partSeat = p, se
	n.ext = featSlice(p.Ext)
	tab := n.ptab[se]
	n.sumRows = n.sumRows[:0]
	for i := range p.Objs {
		po := &p.Objs[i]
		if v := &n.pver[se][po.Idx]; *v != po.Ver {
			row := tab[int(po.Idx)*n.vecW:][:n.vecW]
			clear(row)
			g.BagAdd(row, featSlice(po.Rows))
			*v = po.Ver
		}
		n.sumRows = append(n.sumRows, policynet.Feature{Row: po.Idx, Value: 1})
	}
	clear(n.bagSum)
	netsimd.SumRows(n.bagSum, tab, n.vecW, n.sumRows)
	g.BagAdd(n.bagSum[:], featSlice(p.Fresh))
	pv := &n.pvec[se]
	n.cin, n.miss = n.cin[:len(p.Ents)], n.miss[:0]
	for k := range p.Ents {
		pe := &p.Ents[k]
		ci := &n.cin[k]
		*ci = netsimd.CardIn{Group: pe.Group}
		hit := false
		if pe.Ver != 0 {
			c := 0
			if pe.Castable {
				c = 1
			}
			if pv[pe.Idx][c] == pe.Ver {
				hit = true
				at := (int(pe.Idx)*2 + c) * n.vecW
				ci.Vec = n.pvv[se][at : at+n.vecW]
			}
		}
		if !hit {
			raw := &n.raws[k]
			*raw = pe.Card.Raw
			if pe.Castable {
				raw[feat.EntCastable] = 1
			}
			ci.Raw, ci.Rows = raw[:], featSlice(pe.Card.Rows[:pe.Card.NRows])
			if pe.Ver != 0 {
				n.miss = append(n.miss, int32(k))
			}
		}
	}
}

// evalParts runs the network on prepParts' inputs and stores the card
// vectors it computed, each in its parts slot.
func (n *Net) evalParts(opts []policynet.Option, scores []float32) float32 {
	v := n.w.g.EvalBagExt(&n.sc, n.cin, n.parts.Dense[:], n.bagSum[:], n.ext, opts, scores)
	pv := &n.pvec[n.partSeat]
	for _, k := range n.miss {
		pe := &n.parts.Ents[k]
		c := 0
		if pe.Castable {
			c = 1
		}
		at := (int(pe.Idx)*2 + c) * n.vecW
		copy(n.pvv[n.partSeat][at:at+n.vecW], n.sc.CardVec(int(k), n.vecW))
		pv[pe.Idx][c] = pe.Ver
	}
	return v
}

func featSlice(f []feat.Feature) []policynet.Feature {
	if len(f) == 0 {
		return nil
	}
	return unsafe.Slice((*policynet.Feature)(unsafe.Pointer(&f[0])), len(f))
}

// The cache keys. KeyContent (the default) keys an evaluation by
// dirac's content hashes of what the encoder reads (State.ViewHash of the
// encoding seat, State.DecisionHash, the bot marks), so a hit costs no
// encoding at all; KeyEncoded keys it by a hash of the encoded inputs
// (encode, then look up). Both are exact up to 64-bit collisions; content
// keys are more specific (two positions can encode alike yet hash apart),
// never less. VerifyKeys checks the content key against the encoded one on
// every call (a test switch: it encodes always and allocates a map).
const (
	KeyContent = iota
	KeyEncoded
)

// SetBase records s (the State a search just gave a fresh epoch,
// dirac.State.SetEpoch) as the base its worlds are cloned from: the
// content hash of every object of s, which a clone's unchanged objects
// reuse (search.BaseSetter).
// The content keys read the encoder's object hashes (feat
// Encoder.BaseHashes, HashCache): an object hashed for the key is not
// hashed again for its card view.
func (n *Net) SetBase(s *dirac.State) {
	if !n.NoViewBase {
		n.enc.SetBase(s)
	}
}

func (n *Net) valueKey(s *dirac.State, d *dirac.Decision, seat uint8) uint64 {
	h := s.ViewHashCached(seat, n.enc.BaseHashes(s), n.enc.HashCache())
	return hu(hu(hu(h, uint64(d.Kind)|uint64(d.Sub)<<8|uint64(d.Player)<<16), 0x76616c), n.poolGen)
}

func (n *Net) priorKey(s *dirac.State, d *dirac.Decision, bot []bool) uint64 {
	h := hu(hu(s.ViewHashCached(d.Player, n.enc.BaseHashes(s), n.enc.HashCache()), s.DecisionHash(d)), n.poolGen)
	var w uint64
	m := int(d.N)
	for i := 0; i < m && i < len(bot); i++ {
		if bot[i] {
			w |= 1 << (i & 63)
		}
		if i&63 == 63 {
			h, w = hu(h, w), 0
		}
	}
	return hu(hu(h, w), uint64(m))
}

// verify records that content key ck produced encoded key ek.
func (n *Net) verify(ck, ek uint64) {
	if n.seen == nil {
		n.seen = map[uint64]uint64{}
	}
	if old, ok := n.seen[ck]; ok && old != ek {
		n.KeyConflicts++
	}
	n.seen[ck] = ek
	n.KeyChecks++
}

// Value is the value head's win probability for seat at s (pending
// decision d), read from seat's view (azmcts leafValue: the actor's view
// whoever's decision is pending).
func (n *Net) Value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	v := n.value(s, d, seat)
	if c := n.check; c != nil {
		c.ref.NewWorld()
		rv := c.ref.Value(s, d, seat)
		c.note(math.Abs(float64(v-rv)), 0)
	}
	return v
}

// ValueBatch is search.BatchEvaluator: the batched tree's leaves, valued
// in one call. The pinned gorge netsimd exposes only one-position
// entries (Eval; EvalBag/EvalCards are card sets, not batch), so this is
// the serial value path per position -- the batch seam is here for a
// netsimd batch entry to slot into. It allocates nothing.
func (n *Net) ValueBatch(states []*dirac.State, decs []*dirac.Decision, seat uint8, out []float32) {
	for i := range out {
		out[i] = n.Value(states[i], decs[i], seat)
	}
}

func (n *Net) value(s *dirac.State, d *dirac.Decision, seat uint8) float32 {
	if d.Player == seat && n.same(s, d) {
		return n.lastV
	}
	t := profStart()
	var key uint64
	encoded := false
	if n.Keys == KeyContent {
		key = n.valueKey(s, d, seat)
		if n.VerifyKeys {
			n.state(s, d, seat)
			n.verify(key, hu(n.stateHash(), 0x76616c))
			encoded = true
		}
	} else {
		n.state(s, d, seat)
		key = hu(n.stateHash(), 0x76616c)
		encoded = true
	}
	profFeat(t)
	if e, hit := n.lookup(key); hit {
		n.Hits++
		if d.Player == seat {
			n.remember(s, d, e.v)
		}
		return e.v
	}
	var v float32
	if n.Materialize {
		if !encoded {
			t = profStart()
			n.state(s, d, seat)
			profFeat(t)
		}
		t = profStart()
		v = n.evalFull(nil, nil)
	} else {
		t = profStart()
		n.prepParts(s, d, seat)
		profFeat(t)
		t = profStart()
		v = sigmoid(n.evalParts(nil, nil))
	}
	n.Evals++
	profNet(t)
	if e, _ := n.lookup(key); e != nil {
		e.key, e.ok, e.v, e.n = key, true, v, 0
	}
	if d.Player == seat {
		n.remember(s, d, v)
	}
	return v
}

// encodePriors encodes d's state and options into n.pst and n.popts.
func (n *Net) encodePriors(s *dirac.State, d *dirac.Decision, bot []bool) {
	n.state(s, d, d.Player)
	n.encodeOptions(d, bot)
}

// encodeOptions encodes d's options into n.popts (after a state encode of
// d.Player's view, which built the view they read).
func (n *Net) encodeOptions(d *dirac.Decision, bot []bool) {
	m := int(d.N)
	// options sit at gorge's net-list index/length (feat listpos.go); a
	// priority bot answer orders the appended synthetic casts (bot first)
	botAt := -1
	for i := 0; d.Kind == dirac.DkPriority && i < m && i < len(bot); i++ {
		if bot[i] {
			botAt = i
			break
		}
	}
	fo := n.enc.EncodeOptionsBot(d, n.fopts[:0], botAt)
	for i := 0; i < m; i++ {
		f := &fo[i]
		n.popts[i] = policynet.Option{
			Slots:   featSlice(f.Slots[:f.NSlots]),
			Hashed:  featSlice(f.Hashed[:f.NHashed]),
			Dense:   f.Dense[:],
			EntA:    f.EntA,
			EntB:    f.EntB,
			BotPick: i < len(bot) && bot[i],
		}
	}
}

// encodedPriorKey hashes the encoded n.pst and n.popts[:m].
func (n *Net) encodedPriorKey(m int) uint64 {
	key := n.stateHash()
	for i := 0; i < m; i++ {
		o := &n.popts[i]
		key = hfeat(key, o.Slots)
		key = hfeat(key, o.Hashed)
		key = hfloats(key, o.Dense)
		key = hu(key, uint64(uint32(o.EntA))<<32|uint64(uint32(o.EntB)))
		if o.BotPick {
			key = hu(key, 1)
		}
	}
	return hu(key, uint64(m))
}

// Priors writes the option logits of d (bot marks the bot's options).
func (n *Net) Priors(s *dirac.State, d *dirac.Decision, bot []bool, out []float32) {
	n.priors(s, d, bot, out)
	if c := n.check; c != nil {
		m := int(d.N)
		c.ref.NewWorld()
		c.ref.Priors(s, d, bot, c.buf[:m])
		var dl float64
		for i := 0; i < m; i++ {
			dl = math.Max(dl, math.Abs(float64(out[i]-c.buf[i])))
		}
		c.note(0, dl)
	}
}

// Check is a test hook (EnableCheck): every evaluation is repeated through
// a reference Net -- the materialized encoding, the view built in full, no
// cache -- and the largest differences recorded. It doubles the cost.
type Check struct {
	ref        *Net
	N, Bad     int     // evaluations checked; of them, off by more than 1e-4
	MaxV, MaxL float64 // largest value and logit differences
	buf        [dirac.MaxOptions]float32
}

func (c *Check) note(dv, dl float64) {
	c.N++
	c.MaxV = math.Max(c.MaxV, dv)
	c.MaxL = math.Max(c.MaxL, dl)
	if dv > 1e-4 || dl > 1e-4 {
		c.Bad++
	}
}

// EnableCheck turns the Check hook on and returns it.
func (n *Net) EnableCheck() *Check { return n.EnableCheckWith(n.w) }

// EnableCheckWith is EnableCheck against another network's weights: the
// reference evaluates every position through ref's backend (a netsimd Net
// checked against the same network loaded through gonet measures the two
// backends' arithmetic).
func (n *Net) EnableCheckWith(ref *Weights) *Check {
	r := New(ref, n.enc.Pool())
	r.Materialize, r.NoViewBase, r.Keys, r.cache = true, true, KeyEncoded, nil
	n.check = &Check{ref: r}
	return n.check
}

func (n *Net) priors(s *dirac.State, d *dirac.Decision, bot []bool, out []float32) {
	t := profStart()
	m := int(d.N)
	var key uint64
	encoded := false
	if n.Keys == KeyContent {
		key = n.priorKey(s, d, bot)
		if n.VerifyKeys {
			n.encodePriors(s, d, bot)
			n.verify(key, n.encodedPriorKey(m))
			encoded = true
		}
	} else {
		n.encodePriors(s, d, bot)
		key = n.encodedPriorKey(m)
		encoded = true
	}
	profFeat(t)
	if e, hit := n.lookup(key); hit && int(e.n) == m {
		n.Hits++
		copy(out[:m], e.logits[:m])
		n.remember(s, d, e.v)
		return
	}
	var v float32 // the value probability
	if n.Materialize {
		if !encoded {
			t = profStart()
			n.encodePriors(s, d, bot)
			profFeat(t)
		}
		t = profStart()
		v = n.evalFull(n.popts[:m], out[:m])
	} else {
		t = profStart()
		n.prepParts(s, d, d.Player)
		n.encodeOptions(d, bot)
		profFeat(t)
		t = profStart()
		v = sigmoid(n.evalParts(n.popts[:m], out[:m]))
	}
	n.Evals++
	profNet(t)
	n.remember(s, d, v)
	if e, _ := n.lookup(key); e != nil && m <= cacheOpts {
		e.key, e.ok, e.v, e.n = key, true, v, uint16(m)
		copy(e.logits[:m], out[:m])
	}
}

// evalFull runs the backend on the materialized encoding (n.pst and opts),
// writing the option logits into out; it returns the value probability.
func (n *Net) evalFull(opts []policynet.Option, out []float32) float32 {
	if n.w.g != nil {
		return sigmoid(n.w.g.EvalExt(&n.sc, &n.pst, n.ext, opts, out))
	}
	return sigmoid(n.w.gn.Eval(&n.pst, opts, out))
}

// NewWorld forgets the cached last evaluation (search.WorldReset).
func (n *Net) NewWorld() { n.haveV = false }

// SetPool rebinds the evaluator to another matchup's feat pool (no
// allocation).
func (n *Net) SetPool(pool *feat.Pool) {
	if n.check != nil {
		n.check.ref.SetPool(pool)
	}
	if pool != n.enc.Pool() {
		n.poolGen++
	}
	n.enc.SetPool(pool)
	n.haveV = false
}

// Encoder is the evaluator's feature encoder (diagnostics: its stats).
func (n *Net) Encoder() *feat.Encoder { return n.enc }
