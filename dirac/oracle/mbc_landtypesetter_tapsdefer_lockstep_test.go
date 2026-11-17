package oracle

import "testing"

// CR 305.7 with CR 603.2: Spreading Seas makes an enchanted land an Island, so
// Rikala, Homarid King's "whenever you tap an Island for mana" gate (three
// tide counters) defers that Island too, as gorge's planner does every other
// Island. The kernel left the enchanted Island a plain census unit (its
// land-type-setter view was no gap), priced Dream Thrush ({1}{U}) and a second
// Rikala with it, and listed both where gorge's autopay surface had neither
// (decision 680 of dr-mbc-abab pair 80).
func TestMBCDreamThrushReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "land_type_setter_island_defers_to_the_tap_watcher")
}
