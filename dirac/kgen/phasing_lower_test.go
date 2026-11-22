package kgen

import "testing"

// phasingCarriers print a bare K:Phasing (CR 702.26a) and nothing else kgen
// refuses; each lowers to CardDef.Phasing, which the kernel's untap-step
// phasing action reads (dirac/kw_phasing.go phaseUntap).
var phasingCarriers = []string{
	"Breezekeeper", "Merfolk Raiders", "Teferi's Drake", "Tolarian Drake", "Sandbar Crocodile",
}

func TestPhasingCarriersLower(t *testing.T) {
	for _, name := range phasingCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		if !c.Phasing {
			t.Errorf("%s: CardDef.Phasing not set", name)
		}
	}
}
