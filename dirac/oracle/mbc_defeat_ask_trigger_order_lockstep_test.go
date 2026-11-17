package oracle

import "testing"

// A defeated Siege's "cast it transformed" ask waits out the trigger-order
// ask of the combat damage that defeated it (pool tail-62 focus Zebra Unicorn
// and Invasion of Zendikar, pair 10, mulligans 1, surface autopay): Zebra
// Unicorn triggers needed ordering and the kernel posed the defeat ask after
// the first pick where gorge asked the next pick (CR 603.3b; gorge's
// trigger-order answer resumes the drain and the priority round, and the
// defeat ask comes at the next step head).
func TestMBCDefeatAskTriggerOrderReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "defeat_ask_waits_out_trigger_order")
}
