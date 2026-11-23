package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// windowPricesRestr prices a restricted mana ability only when its cost is a plain {T}
// (restrSrcAb). gorge's cast-window probe also prices one whose cost adds a generic
// part or a life payment (castWindowProbeUnits), so a supported card carrying such an
// ability would leave dirac's window census short of gorge's. kgen refuses them today
// ("mana restriction on a PayLife source", and the other costs fall to the same
// refusals), which keeps std-msh's Ronin, Shadow Stalker and std-sos's Great Hall of
// the Biblioplex out of every table; no pool deals one. A pool that gains such a
// card must price it in windowPricesRestr before it joins the list. A restricted
// {T}, Pay N life one of a fixed colour (kop.RestrictedLifeShape) is priced there and
// planned as a last-resort unit (mana_restrictedcost.go), and so is a restricted
// {T}, Sacrifice this (kop.RestrictedSacSelfShape, Jeweled Lotus; restrSrcAb admits
// both). A restricted {T}, Sacrifice another (kop.SacXRestrManaAb, Master of Dark
// Rites) is no unit the window prices on either side: gorge's census defers it like
// the unrestricted sacrifice-another shape, and windowCensusComplete reports the gap
// (mana_tapanother.go funds the play it pays), nor is a restricted tap-another
// (kop.RestrictedTapXOneShape, Grand Architect): its tapX cost is the census gap
// windowCensusComplete reads first (tapXCensusGap, gorge castWindowSourceExcluded).
func TestNoSupportedRestrictedManaAbilityBeyondPlainTap(t *testing.T) {
	t.Parallel()
	for _, name := range cardsettest.SynthNames(t) {
		set := cardsettest.Synth(t, name)
		tb, err := set.Pool()
		if err != nil {
			t.Fatal(err)
		}
		sup := set.Supported()
		for i := range tb.Cards {
			c := &tb.Cards[i]
			if i < len(sup) && !sup[i] {
				continue
			}
			for j := range c.Abilities {
				a := &c.Abilities[j]
				if a.Kind == kop.AbMana && (a.ManaRestrict != 0 || a.ManaRestrictAb != 0) && a.Cost.Flags != kop.CostTap &&
					!kop.RestrictedLifeShape(a) && !kop.RestrictedSacSelfShape(a) && !kop.SacXRestrManaAb(a) && !kop.RestrictedTapXOneShape(a) {
					t.Errorf("%s: %s ability %d is a restricted mana ability with a cost beyond {T}: price it in windowPricesRestr", name, c.Name, j)
				}
			}
		}
	}
}
