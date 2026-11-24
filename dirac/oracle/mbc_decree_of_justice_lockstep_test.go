package oracle

import "testing"

// CR 903.4f with CR 601.2b: a cycled Decree of Justice's "you may pay {X}" is
// announced against what the payer's sources can make. With no commander, a
// Command Tower makes no mana, so a payer holding three Plains, a Command
// Tower and a Quirion Elves can announce X up to 4, not 5. Gorge's per-source
// census priced the Tower as a unit and offered X = 5 (decision 458 of 458 in
// dr-mbc-bbaa pair 143).
func TestMBCDecreeOfJusticeReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "trigger_pay_x_bound_skips_empty_commander_identity")
}
