package oracle

import "testing"

// CR 106.1, 605.3b: Elven Chorus gives every creature "{T}: Add one mana of
// any color", and Gyre Sage already makes {G} per +1/+1 counter. A creature
// taps once, so Gyre Sage and Nettletooth Djinn make two black mana at most
// beside three green-only lands: Halo Hunter ({2}{B}{B}{B}) is not castable,
// and gorge doesn't offer it (decision 323 of tail-51 pair 111).
func TestMBCHaloHunterReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "granted_any_beside_counter_mana_host")
}
