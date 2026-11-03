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
// card must price it in windowPricesRestr before it joins the list.
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
				if a.Kind == kop.AbMana && (a.ManaRestrict != 0 || a.ManaRestrictAb != 0) && a.Cost.Flags != kop.CostTap {
					t.Errorf("%s: %s ability %d is a restricted mana ability with a cost beyond {T}: price it in windowPricesRestr", name, c.Name, j)
				}
			}
		}
	}
}
