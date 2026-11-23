package dirac

import (
	"math/bits"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// reflAmtFaceReachable is faceReachable's reflected Amount$ arm (Incubation
// Druid: "{T}: Add one mana of any type that a land you control could
// produce", three of that type with a +1/+1 counter). The cast window's avail
// holds such a source as {C} alone (availCA, gorge's potential bound), but
// gorge's hybrid pip ask prices each face through the CR 601.2g window probe
// (rules castWindowProbeUnits), which replaces the source with one alternative
// per reflected colour carrying the whole evaluated amount (CR 605.1a). A face
// is therefore reachable when swapping the source's {C} units for its amount in
// one reflected colour (a tapped Swamp's {B} counts: "could produce") affords
// it. The source's one colour ask names all its mana, so each colour is tried
// on its own.
func (s *State) reflAmtFaceReachable(k *prep, p uint8, av *avail, r *kop.ManaCost, extra int) bool {
	if k.gen == nil || !k.gen.anyComboAny || !k.gen.uses.has(useReflAmt) {
		return false
	}
	for b := s.caGaps(k, p); b.any(); b = b.rest() {
		sl := b.first()
		if !k.isReflAmt(s.cardAt(sl)) || s.pickCommittedSrc(k, sl) {
			continue
		}
		n := s.caAmount(k, sl)
		for m := s.reflAmtCols(k, sl); m != 0; m &= m - 1 {
			t := *av
			t.takeWide(uint8(kop.ColC), uint16(n))
			t.addUnits(uint8(1)<<uint(bits.TrailingZeros8(m)), uint8(n))
			if s.faceReachable(k, p, &t, r, extra) {
				return true
			}
		}
	}
	return false
}
