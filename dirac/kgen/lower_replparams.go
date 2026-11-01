package kgen

// replIsPresent reads an ETB replacement's IsPresent$ (Ascendant
// Packleader's K:etbCounter ... IsPresent$ Permanent.YouCtrl+cmcGE4;
// gorge's replacement IsPresent gate: one or more battlefield permanents
// match, the entering card not yet among them). PresentCompare$ /
// PresentZone$ beside it, or a second condition, refuse.
func (lc *lowerCtx) replIsPresent(rp *params, rl *lRepl) error {
	ip, ok := rp.get("IsPresent")
	if !ok {
		return nil
	}
	for _, k := range []string{"PresentCompare", "PresentZone", "PresentPlayer"} {
		if _, ok := rp.get(k); ok {
			return refuse("replacement IsPresent$ with %s$", k)
		}
	}
	if rl.Cond.Kind != cNone {
		return refuse("two replacement conditions")
	}
	f, err := lc.filter(ip)
	if err != nil {
		return err
	}
	rl.Cond = lCond{Kind: cPresent, F: f, Op: opGE, A: cst(1)}
	return nil
}
