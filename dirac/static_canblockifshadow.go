package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Printed S:Mode$ CanBlockIfShadow statics (kop.StatCanBlockIfShadow, CR
// 702.28b; gorge shadowBlockLifted in engine/rules/combat/block_statics.go):
// a blocker matching Filter can block a shadow attacker matching Filter2 as
// though the shadow rule were absent. Filter is ValidBlocker$, Filter2
// ValidAttacker$; an absent filter matches any creature. The shared gates
// ride Cond2 (gorge's StaticGateHolds; gorge skips the whole lift for a
// gated static whose condition does not hold).

// useCanBlkShadow arms shadowLifted: a table holding a StatCanBlockIfShadow
// static scans the statics in the shadow branch of the block check.
var useCanBlkShadow = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatCanBlockIfShadow) })

// shadowLifted reports whether a live StatCanBlockIfShadow static lets
// blocker b block shadow attacker a (gorge shadowBlockLifted).
func (s *State) shadowLifted(k *prep, b, a Slot) bool {
	if k.gen == nil || !k.gen.uses.has(useCanBlkShadow) {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		ab := st.a
		if ab.Kind != kop.AbStatic || ab.Static.Kind != kop.StatCanBlockIfShadow || !s.statCond(k, st) {
			return true
		}
		if ab.Static.Filter != 0 && !s.matchPermGen(k, ab.Static.Filter, b, st.ctrl, st.sl, layerFull) {
			return true
		}
		if ab.Static.Filter2 != 0 && !s.matchPermGen(k, ab.Static.Filter2, a, st.ctrl, st.sl, layerFull) {
			return true
		}
		hit = true
		return false
	})
	return hit
}
