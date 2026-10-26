package netsimd

// SetGeneral forces a production-layout net through trunkBagN (the general
// loops) instead of trunkBag1, and reports whether it had the layout.
func SetGeneral(g *Net, general bool) (was bool) {
	was = g.one || len(g.blocks) == 1 && len(g.polHid) == 1 && len(g.valHid) == 0
	g.one = was && !general
	return was
}
