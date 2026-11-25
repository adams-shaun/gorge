package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// mostProminentType is ExMostProminentCreatureType (gorge
// mostProminentCreatureType, engine/effects/count_cov3.go): the size of the
// largest group of battlefield objects matching F that share one current
// creature type. An object with every creature type (a Changeling, CR 702.73a)
// joins every group and is counted once on top of the largest one; its own
// other types are not tallied. Cold, out of line.
//
//go:noinline
func (s *State) mostProminentType(k *prep, x *kop.Expr, ctx evalCtx) int {
	var cnt [kop.MaxSubtypes]int
	all := 0
	for m := s.occ; m.any(); m = m.rest() {
		sl := m.first()
		if !s.matchPermGen(k, x.F, sl, ctx.ctrl, ctx.srcSl, layerFull) {
			continue
		}
		if s.hasSubGen(k, sl, subOutside) {
			all++
			continue
		}
		for i := 0; i < len(k.t.Subtypes); i++ {
			if k.gen.creatSub.Has(i) && s.hasSubGen(k, sl, uint8(i)) {
				cnt[i]++
			}
		}
	}
	best := 0
	for i := 0; i < len(k.t.Subtypes); i++ {
		if cnt[i] > best {
			best = cnt[i]
		}
	}
	return best + all
}
