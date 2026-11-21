package kgen

func init() { kwHandlers["Vanishing"] = (*lowerCtx).lowerVanishingKw }

// lowerVanishingKw lowers K:Vanishing[:N] (CR 702.63a): gorge's kwVanishing
// (cards/kw_vanishing.go) expands it into the enters-with-N-time-counters
// replacement (a numeric line only), the upkeep trigger removing a time
// counter and the last-counter trigger (Mode$ CounterRemoved, CounterType$
// TIME, NewCounterAmount$ 0) sacrificing the permanent, each lowered from its
// own line. The head is also read by rules code (trigmatch/counters.go:205):
// the last-counter trigger fires on a positive-to-zero transition, not on
// every CounterChange whose clamped result is zero (the TIME-removal LKI,
// Engine.emit). The kernel's counter-removed event fires only for a removal
// of at least one counter (dirac/trig_counterremoved.go fireCtrRemoved: n is
// clamped to the counters held, a removal of none fires nothing), so the left
// == 0 test is the same transition. ctrRemovedTrigger admits the battlefield
// time-counter line only when it carries the Vanishing tag. A face whose
// expansion a printed line suppressed refuses.
func (lc *lowerCtx) lowerVanishingKw(node, _ string) {
	line := kwLineOf(node)
	for _, suffix := range [...]string{"#upkeep", "#last-counter"} {
		found := false
		for _, t := range lc.face.Triggers {
			found = found || t.Params["KeywordLine"] == line+suffix
		}
		if !found {
			lc.refuseNode(node, "keyword %q without its expansion", "Vanishing")
			return
		}
	}
}
