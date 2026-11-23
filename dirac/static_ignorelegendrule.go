package dirac

import "github.com/adams-shaun/gorge/dirac/kop"

// Printed S:Mode$ IgnoreLegendRule statics (kop.StatIgnoreLegendRule, CR
// 704.5j; gorge legendGroups' legendRuleExempt skip in engine/rules/sba.go):
// a permanent matching a live static's Filter, resolved with the static's
// controller and source, is exempt from the legend rule while the static's
// gate holds — Mirror Gallery's blanket form (no ValidCard$), Council of
// Reeds' creatures-you-control, Brothers Yamazaki's "exactly two named"
// gate. The exemption skips the candidate from the duplicate scan entirely,
// as gorge's does: a group whose every member is exempt rules nothing.

// useLgExempt arms legendExemptGen: a table holding a StatIgnoreLegendRule
// static scans the statics in legendGroup's candidate walk.
var useLgExempt = newGenUse(func(t *kop.Table) bool { return tableHasStatic(t, kop.StatIgnoreLegendRule) })

// legendExemptGen reports whether a live StatIgnoreLegendRule static exempts
// the permanent at sl from the legend rule (gorge legendRuleExempt). Its own
// scan, not staticFlagGen: the blanket form (Mirror Gallery) carries no
// ValidCard$, and matchPermGen's empty filter matches nothing.
func (s *State) legendExemptGen(k *prep, sl Slot) bool {
	if k.gen == nil || !k.gen.uses.has(useLgExempt) {
		return false
	}
	hit := false
	s.eachStatic(k, func(st statSrc) bool {
		a := st.a
		if a.Kind == kop.AbStatic && a.Static.Kind == kop.StatIgnoreLegendRule && s.statCond(k, st) &&
			(a.Static.Filter == 0 || s.matchPermGen(k, a.Static.Filter, sl, st.ctrl, st.sl, layerFull)) {
			hit = true
			return false
		}
		return true
	})
	return hit
}
