package oracle

import (
	"testing"
)

// CR 613.1f layer-3 copy: Hazel's Brewmaster exiled Naga Vitalist, so Foods
// its controller controls gain "{T}: Add one mana of any type a land you
// control could produce" (a noncreature artifact has no summoning-sickness
// limit on {T}). dirac's gainedManaPlanned puts kop.AfManaReflect in its
// unmodelled mask, so it never offered that Food as a third mana source. At
// decision 268 p1 held two untapped lands, Hazel's Brewmaster and the Food
// token: gorge offered cast(Yahenni, Undying Partisan) and activate(Ecstatic
// Awakener) off the reflected Food, dirac offered neither.
func TestMBCYahenniUndyingPartisanReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "gained_reflected_mana_food")
}
