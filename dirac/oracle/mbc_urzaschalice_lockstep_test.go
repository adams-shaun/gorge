package oracle

import "testing"

// CR 117.3 / 601.2g: Urza's Chalice's "you may pay {1}" is payable when the
// payer's only untapped mana source is Sage of the Maze ("{T}: Add two mana in
// any combination of colors", a Combo Any census gap). gorge's trigger-cost
// ask offers pay and decline; the kernel's affordability gate priced the
// window without the Combo Any source and offered decline only (decision 324
// of tail-36-c000 pair 79).
func TestMBCUrzaSChaliceReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "combo_any_source_pays_trigger_cost")
}
