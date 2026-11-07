package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// counterKinds is ExCounterKinds: the distinct counter kinds with a positive
// count among the battlefield objects matching F (gorge
// Count$DifferentCounterKinds_<spec>). Every kind lives in a perm's +1/+1
// field, its general slots or the overflow table. gorge's markers (the
// regeneration shield, the deathtouch mark) are perm flags here, never a
// counter kind, and a Class level (CtrLevel) is no counter in gorge, so
// kop.IsCounterKind skips it.
//
//go:noinline
func (s *State) counterKinds(k *prep, x *kop.Expr, ctx evalCtx) int {
	var seen ctrKindSet
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPermGen(k, x.F, sl, ctx.ctrl, ctx.srcSl, layerFull) {
			continue
		}
		p := &s.perms[sl]
		if p.p1 > 0 {
			seen.mark(int(kop.CtrP1P1))
		}
		for i := range p.ck {
			if p.cn[i] > 0 {
				seen.mark(int(p.ck[i]))
			}
		}
		if p.ck[ctrSlots-1] != 0 {
			for _, e := range s.ctrOverOf(p.obj, 0) {
				if e.n > 0 {
					seen.mark(int(e.kind))
				}
			}
		}
	}
	n := 0
	for _, w := range seen {
		n += bits.OnesCount64(w)
	}
	return n
}

// ctrKindSet is a bit per counter kind; the kinds outnumber one word.
type ctrKindSet [(int(kop.NumCounterKinds3) + 63) / 64]uint64

// mark adds kind c, unless c is no real counter kind.
func (m *ctrKindSet) mark(c int) {
	if kop.IsCounterKind(c) {
		m[c>>6] |= 1 << (c & 63)
	}
}
