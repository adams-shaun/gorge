package oracle

import "testing"

// CR 205.3m with CR 701.20a: Tajuru Paragon's kicked enters trigger digs six
// and may take a creature card that shares a creature type with it. Its Elf
// type is the only one it shares with Greenside Watcher (Elf Druid), so the
// matchup's table must keep Elf even though no filter names it. The kernel's
// table dropped it, reading Tajuru as a bare Warrior, and left Greenside
// Watcher out of the choice (decision 559 of tail-52 pair 3).
func TestMBCGreensideWatcherReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dig_shares_creature_type_keeps_printed_subtypes")
}
