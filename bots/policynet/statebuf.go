package policynet

import (
	"sync"

	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/engine/view"
)

// StateBuf is reusable storage for a state encoding (EncodeStateWithInto):
// the dense vector, the sparse bag, the entity list and the slabs the
// entity cards' Raw and Rows are carved from. A State encoded into a buf
// aliases it and is valid until the buf's next encode or release. The nil
// *StateBuf allocates fresh storage for every encode (EncodeStateWith).
type StateBuf struct {
	dense  []float32
	sparse []Feature
	cards  []EntityCard
	raw    []float32
	rows   []Feature
}

var stateBufs = sync.Pool{New: func() any { return new(StateBuf) }}

// AcquireStateBuf returns a pooled StateBuf; ReleaseStateBuf returns it once
// nothing reads a State encoded into it any longer.
func AcquireStateBuf() *StateBuf  { return stateBufs.Get().(*StateBuf) }
func ReleaseStateBuf(b *StateBuf) { stateBufs.Put(b) }

// EncodeStateWithInto is EncodeStateWith(fs, *v, seat, diag) encoded into
// buf's storage: the same values, with no allocation once buf has grown.
func EncodeStateWithInto(buf *StateBuf, fs FeatureSet, v *view.View, seat state.PlayerID, diag *Diag) State {
	return encodeStateWith(buf, fs, v, seat, diag)
}

// denseBuf is a zeroed n-float dense vector.
func (b *StateBuf) denseBuf(n int) []float32 {
	if b == nil {
		return make([]float32, n)
	}
	if cap(b.dense) < n {
		b.dense = make([]float32, n)
	}
	d := b.dense[:n]
	clear(d)
	return d
}

// sparseBuf is an empty bag with the buf's capacity (nil without a buf).
func (b *StateBuf) sparseBuf() []Feature {
	if b == nil {
		return nil
	}
	return b.sparse[:0]
}

// keepSparse records s (possibly regrown) as the buf's bag storage.
func (b *StateBuf) keepSparse(s []Feature) {
	if b != nil {
		b.sparse = s
	}
}

// entitySlabs are the entity list's storage for total cards: an empty card
// list, total*rawW zeroed raw floats, an empty rows slab.
func (b *StateBuf) entitySlabs(total, rawW int) ([]EntityCard, []float32, []Feature) {
	if b == nil {
		return make([]EntityCard, 0, total), make([]float32, total*rawW), make([]Feature, 0, 2*total)
	}
	if cap(b.cards) < total {
		b.cards = make([]EntityCard, 0, total+total/2)
	}
	if n := total * rawW; cap(b.raw) < n {
		b.raw = make([]float32, n, n+n/2)
	}
	if cap(b.rows) < 2*total {
		b.rows = make([]Feature, 0, 2*total+total)
	}
	raw := b.raw[:total*rawW]
	clear(raw)
	return b.cards[:0], raw, b.rows[:0]
}

// keepEntity records the (possibly regrown) entity storage.
func (b *StateBuf) keepEntity(cards []EntityCard, rows []Feature) {
	if b != nil {
		b.cards, b.rows = cards, rows
	}
}
