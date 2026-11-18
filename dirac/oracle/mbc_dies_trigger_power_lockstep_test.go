package oracle

import "testing"

// CR 608.2h / 603.10a: Goblin Fireleaper's "when it dies, it deals damage equal
// to its power" reads the power it last had on the battlefield, pump included
// (its {1}{R} +1/+0 until end of turn). Fireleaper attacked pumped to 2 power,
// traded with an Aven Initiate (3/2) and its trigger then dealt 1 to the other
// Initiate where dirac, reading last-known power 2, dealt 2 (decision 1020 of
// 1020; tail-45-c000 pair 1, filed as the Dreamspoiler Witches options
// divergence before an earlier fix moved the first divergence here).
func TestMBCDreamspoilerWitchesReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dies_trigger_power_last_known")
}
