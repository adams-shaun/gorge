package oracle

import (
	"testing"
)

// CR 604.3: a characteristic-defining ability functions in every zone, so
// Phyrexian Ingester's "+X/+Y, where X is the exiled creature card's power and
// Y is its toughness" reads an exiled Abominable Treefolk through its P/T CDA
// (a */* whose value is the number of snow permanents its controller controls)
// for BOTH heads: the card sits in exile at 1/1 (the controller's one snow
// permanent, the Treefolk itself now gone), so the Ingester is a 4/4. Gorge's
// count-ref toughness read once skipped the CDA branch its power read carries
// (engine/effects/count_ref_property.go), leaving the Ingester a 4/3 while the
// kernel read 4/4 (ev-tail-70-c000 pair 62, seed 867923, decision 475:
// projection Cards[25].Toughness, ref=3 kernel=4).
func TestMBCPhyrexianIngesterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "offzone_cda_toughness")
}
