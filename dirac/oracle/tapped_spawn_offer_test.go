package oracle

// munitionsOfferWatch is the tail policy of TestTappedSpawnPaysTheCastLockstep:
// it plays FirstPolicy and records whether the first decision past the script
// (the game's decision at the original witness's divergence) offered a cast of
// Makeshift Munitions, the priced-through-a-tapped-Spawn scenario.
type munitionsOfferWatch struct {
	first   Policy
	n       int
	at      int
	offered bool
}

func (w *munitionsOfferWatch) Pick(c *PickCtx) Canon {
	if w.first == nil {
		w.first = FirstPolicy()
	}
	if w.n == 0 {
		w.at = c.Seq
		for _, o := range c.D.Options {
			if o.Kind() == ActCast && c.G != nil && refCardName(c.G, o.Src()) == "Makeshift Munitions" {
				w.offered = true
			}
		}
	}
	w.n++
	return w.first.Pick(c)
}
