package netsimd

import (
	"sync/atomic"
	"unsafe"

	"github.com/adams-shaun/gorge/bots/policynet"
)

// cards.go is the precomputed-card API: a caller that knows which cards are
// unchanged between evaluations (a search over one matchup's card pool)
// computes a card's vector once and hands it back instead of its features.
//
// A card vector is relu(card_phi(relu(card_raw(raw) + card_rows(bag(rows))))).
// Its identity half, card_rows(bag(rows)), depends only on the card's
// identity rows (policynet EntityCard.Rows: the card's name and spell API),
// so it is a per-card constant; the rest reads the card's dynamic raw
// features. CardIdentity computes the half, CardVector the whole vector
// from the half, and EvalCards runs the network on a mix of precomputed
// vectors and cards it computes itself. Every path is bit-identical to Eval
// (TestPrecomputedCardsMatchEval): the identity half is the same
// single-rounded sum Eval adds to card_raw's output.

// CardWidth is the width of a card vector and of its identity half.
func (g *Net) CardWidth() int { return g.cd }

// CardIdentity writes the identity half of a card's vector -- card_rows over
// the bag of its identity rows -- to dst (CardWidth floats).
func (g *Net) CardIdentity(sc *Scratch, rows []policynet.Feature, dst []float32) {
	cd, e := g.cd, g.e
	sc.bag32 = growF(sc.bag32, e)
	sc.u = growF(sc.u, cd)
	clear(sc.bag32)
	if len(rows) > 0 {
		bag(&sc.bag32[0], &g.table[0], (*feat)(unsafe.Pointer(&rows[0])), len(rows), tableRows, e)
	}
	gemvT(&g.cardRows.wt[0], &sc.bag32[0], &g.zero[0], &sc.u[0], e, cd, false)
	copy(dst[:cd], sc.u)
}

// CardVector writes the vector of a card with raw features raw and identity
// half ident (CardIdentity's output) to dst (CardWidth floats).
func (g *Net) CardVector(sc *Scratch, raw, ident, dst []float32) {
	sc.ent1 = growF(sc.ent1, g.cd)
	g.cardSplit(sc, raw, ident, &sc.ent1[0])
	copy(dst[:g.cd], sc.ent1)
}

// cardSplit is cardFull with the identity half precomputed: Eval's
// gemvT(card_rows, bag, acc=u) is u + Σ, the same sum CardIdentity rounds
// once, so u + ident is bit-identical.
func (g *Net) cardSplit(sc *Scratch, raw, ident []float32, ent *float32) {
	cd := g.cd
	sc.raw = growF(sc.raw, g.rawW)
	sc.u = growF(sc.u, cd)
	n := copy(sc.raw, raw)
	clear(sc.raw[n:])
	gemvT(&g.cardRaw.wt[0], &sc.raw[0], &g.cardRaw.b[0], &sc.u[0], g.rawW, cd, false)
	u := sc.u[:cd]
	for i := range u {
		v := u[i] + ident[i]
		if v < 0 {
			v = 0
		}
		u[i] = v
	}
	gemvT(&g.cardPhi.wt[0], &sc.u[0], &g.cardPhi.b[0], ent, cd, cd, true)
}

// CardIn is one entity card for EvalCards: its pooling group and, in order
// of preference, its precomputed vector (Vec, CardWidth floats), or its raw
// features with a precomputed identity half (Raw + Ident), or its raw
// features and identity rows as Eval reads them (Raw + Rows).
type CardIn struct {
	Group uint8
	Vec   []float32
	Raw   []float32
	Ident []float32
	Rows  []policynet.Feature
}

// EvalCards is Eval over cards given as CardIn: the same logits, bit for
// bit, with every precomputed vector or identity half used instead of
// recomputed. The vectors it computed are readable with sc.CardVec until
// the next evaluation on sc (a caller stores them for the next call). The
// card cache Load was given is not consulted.
func (g *Net) EvalCards(sc *Scratch, cards []CardIn, dense []float32, sparse []policynet.Feature, opts []policynet.Option, scores []float32) float32 {
	nc := g.cardsIn(sc, cards)
	return g.trunk(sc, nc, dense, sparse, opts, scores)
}

// cardsIn fills sc's card vectors and groups from cards.
func (g *Net) cardsIn(sc *Scratch, cards []CardIn) int {
	cd := g.cd
	nc := len(cards)
	sc.ent = growF(sc.ent, max(nc, 1)*cd)
	sc.groups = growG(sc.groups, nc)
	for k := range cards {
		c := &cards[k]
		sc.groups[k] = c.Group
		ent := sc.ent[k*cd : (k+1)*cd]
		switch {
		case c.Vec != nil:
			copy(ent, c.Vec[:cd])
		case c.Ident != nil:
			g.cardSplit(sc, c.Raw, c.Ident, &ent[0])
		default:
			g.cardFull(sc, c.Raw, c.Rows, &ent[0])
		}
	}
	return nc
}

// BagWidth is the width of the sparse bag's row sum (the table width).
func (g *Net) BagWidth() int { return g.e }

// BagAdd adds the sparse rows' table sum, Σ value·table[row] over rows
// below the table size, to dst (BagWidth floats): the bag Eval computes
// over a state's sparse entries, which is linear, so a caller may sum it
// in parts (each part's rows summed in order, the parts then added).
func (g *Net) BagAdd(dst []float32, rows []policynet.Feature) {
	if len(rows) > 0 {
		bag(&dst[0], &g.table[0], (*feat)(unsafe.Pointer(&rows[0])), len(rows), tableRows, g.e)
	}
}

// SumRows adds Σ value·table[row·width : (row+1)·width] over rows to dst
// (width floats): the bag kernel over a caller's own table (width 32 or
// 64, the table width·rows long). A caller keeping per-part
// sums in one table adds them up with it (value 1: exactly dst + row, in
// order).
func SumRows(dst, table []float32, width int, rows []policynet.Feature) {
	if width != 32 && width != 64 {
		panic("netsimd: SumRows width must be 32 or 64")
	}
	if len(rows) == 0 {
		return
	}
	bag(&dst[0], &table[0], (*feat)(unsafe.Pointer(&rows[0])), len(rows), len(table)/width, width)
}

// EvalBag is EvalCards with the state's sparse bag given as its row sum
// (BagWidth floats, BagAdd's) instead of its entries. Summed in another
// order than Eval sums the sorted entries, the logits agree with Eval's to
// float rounding, not bit for bit.
func (g *Net) EvalBag(sc *Scratch, cards []CardIn, dense, bagSum []float32, opts []policynet.Option, scores []float32) float32 {
	nc := g.cardsIn(sc, cards)
	return g.trunkBag(sc, nc, dense, nil, bagSum, opts, scores)
}

// CardVec is card k's vector from the last evaluation on sc (CardWidth
// floats; a view of the scratch, valid until its next evaluation).
func (sc *Scratch) CardVec(k, width int) []float32 { return sc.ent[k*width : (k+1)*width] }

// CardStore is a fixed table of card vectors (or identity halves) indexed
// by a slot the caller owns -- a card id times the caller's variants, say.
// It never allocates after NewCardStore, and Get and Put are safe for
// concurrent use: a slot is written once, by whichever writer claims it
// first, and is readable once that write completes. Two writers of one
// slot must write the same vector (a slot names its inputs exactly);
// the second write is dropped.
type CardStore struct {
	w     int
	vecs  []float32
	state []atomic.Uint32 // per slot: 0 empty, 1 being written, 2 ready
}

const (
	slotEmpty uint32 = iota
	slotWriting
	slotReady
)

// NewCardStore returns an empty store of slots vectors of g's card width.
func NewCardStore(g *Net, slots int) *CardStore {
	return &CardStore{w: g.cd, vecs: aligned(slots * g.cd), state: make([]atomic.Uint32, slots)}
}

// Slots is the store's capacity.
func (c *CardStore) Slots() int { return len(c.state) }

// Get returns slot's vector (a read-only view), or nil when the slot is
// empty or out of range.
func (c *CardStore) Get(slot int) []float32 {
	if uint(slot) >= uint(len(c.state)) || c.state[slot].Load() != slotReady {
		return nil
	}
	return c.vecs[slot*c.w : (slot+1)*c.w : (slot+1)*c.w]
}

// Put stores v (CardWidth floats) in slot unless it is already written or
// being written, and reports whether this call wrote it.
func (c *CardStore) Put(slot int, v []float32) bool {
	if uint(slot) >= uint(len(c.state)) || !c.state[slot].CompareAndSwap(slotEmpty, slotWriting) {
		return false
	}
	copy(c.vecs[slot*c.w:(slot+1)*c.w], v[:c.w])
	c.state[slot].Store(slotReady)
	return true
}

// Reset empties every slot. Not safe concurrently with Get or Put.
func (c *CardStore) Reset() {
	for i := range c.state {
		c.state[i].Store(slotEmpty)
	}
}
