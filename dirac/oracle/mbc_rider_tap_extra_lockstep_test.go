package oracle

import "testing"

// CR 106.12 / CR 605.1: Remote Farm ({T}, remove a depletion counter: add {W}{W})
// enchanted by the opponent's Wild Growth ("whenever enchanted land is tapped
// for mana, its controller adds an additional {G}") makes {W}{W}{G}. gorge
// prices the extra {G} into the source, so the {G} spells Eldrazi Repurposer,
// Gyre Sage and the rest are castable; the kernel's bounded-rider source (a
// depletion land, mana_rider.go) priced only the {W}{W} and offered no cast
// (tail-51-c001 pair 74, decision 193: options options).
func TestMBCEldraziRepurposerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rider_source_tap_extra")
}
