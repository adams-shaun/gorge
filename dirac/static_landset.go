package dirac

import (
	"sort"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// LandTypeWords is landTypeWords (static_landtypesetter.go), sorted, for the
// test that holds it to gorge's vocabulary.
func LandTypeWords() []string {
	out := make([]string, 0, len(landTypeWords))
	for w := range landTypeWords {
		out = append(out, w)
	}
	sort.Strings(out)
	return out
}

// lossTs is the timestamp o's loss of abilities takes in layer 6: the record's,
// but a CR 305.7 land-type setter's (kop.Override.SetLandSub: Dream Thrush's
// RemoveLandTypes$ Animate, beside the attached statics) is zero. gorge's loss
// is part of the layer-4 effect, before any layer-6 grant, so no grant is older
// than it (abilityloss.go abilityLoss: lost with a zero timestamp), and it
// removes no keyword an earlier effect granted (loseAllOnset is not run for
// it). The rest of the setter's reading is static_landtypesetter.go.
func lossTs(o *kop.Override, ts uint16) uint16 {
	if o.SetLandSub {
		return 0
	}
	return ts
}
