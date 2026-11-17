package kgen

// ctrAddedAllTrigger lowers Mode$ CounterAddedAll, the batch watcher over one
// or more other permanents ("whenever you put one or more counters on one or
// more ..."; gorge trigmatch counterAddedAllMatches, kernel
// dirac/trig_counteraddedall.go). Valid$ is the recipient filter,
// CounterType$ the counter kind and ValidSource$ You the putter. The kernel
// event TrigCounterAddedAll is one trigger per counter-adding action with the
// batch total, so Bioessence Hydra's "put that many" reads the whole batch.
// ActivationLimit$ 1 is the generic once-per-turn latch (TfOncePerTurn added
// by lowerTriggered's tail). ValidCard$ and the CounterTypeAddedAll-only
// ValidObject$ stay refused (unread), and FirstTime$ (a CounterTypeAddedAll
// reading) refuses by name.
func (lc *lowerCtx) ctrAddedAllTrigger(tp *params, tr *lTrigger, readF func(string) (*lFilter, error)) error {
	var err error
	if tr.F, err = readF("Valid"); err != nil {
		return err
	}
	ct := tp.str("CounterType")
	if ct == "" {
		return refuse("CounterAddedAll without CounterType")
	}
	tr.Step = ct
	if v, ok := tp.get("FirstTime"); ok {
		return refuse("CounterAddedAll FirstTime$ %q", v)
	}
	if vs, ok := tp.get("ValidSource"); ok {
		if vs != "You" {
			return refuse("CounterAddedAll ValidSource %q", vs)
		}
		tr.Whose = whoYou
	}
	return nil
}
