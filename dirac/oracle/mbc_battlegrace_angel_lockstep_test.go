package oracle

import "testing"

// CR 702.26b: a phased-out permanent is treated as though it doesn't exist.
// Smoldering Egg, phased out by Haystack, no longer reads the mana spent on a
// spell, so gorge's cast planner still prices Battlegrace Angel, Living Wall
// and Firefist Adept (it declined every cast as shape:mana_spent_reader while
// the Egg was out) and the kernel's offers match (decision 869, pool
// tail-29-c001 pair 122, autopay).
func TestMBCBattlegraceAngelReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "phased_out_spend_reader_keeps_casts_planned")
}
