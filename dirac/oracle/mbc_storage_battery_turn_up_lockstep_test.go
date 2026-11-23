package oracle

import "testing"

// A storage land (Fountain of Cho: "{T}, Remove any number of storage
// counters: Add {W} for each") holding counters beside four plain sources
// funds a keyword turn-up the lands alone cannot (Basilica Stalker's
// {4}{B}, Bubble Smuggler's {5}{U}). The recorded game (tail-67 pair 44)
// stops at the priority decision after Alley Assailant turned face up, with
// two Coveted Falcons, a Basilica Stalker and a Bubble Smuggler face down.
func TestMBCBasilicaStalkerReprosLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	stdReproNamed(t, "storage_battery_turn_up_offer")
}
