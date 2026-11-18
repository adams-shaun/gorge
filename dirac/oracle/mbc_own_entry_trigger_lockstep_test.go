package oracle

import "testing"

// CR 603.6a: a permanent's own enters-the-battlefield ability sees the permanent
// entering. Brainstealer Dragon, owned by one player and cast from the other's
// exile through the other Dragon's "may play", enters under the caster's control
// and triggers its own "a nonland permanent an opponent owns enters under your
// control" beside the caster's other Dragons. Gorge matched the self-entry
// against its stack-zone last-known information, where it isn't a permanent, so
// it offered two trigger_order options where the kernel offered three
// (decision 719 of dr-mbc-aabb pair 14, autopay).
func TestMBCBrainstealerDragonReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "own_entry_trigger_of_stolen_permanent")
}
