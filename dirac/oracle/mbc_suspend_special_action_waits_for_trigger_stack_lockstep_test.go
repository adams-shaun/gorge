package oracle

import "testing"

// CR 702.62a, 307.1, 117.1a with CR 605.3b: the suspend special action follows
// the timing of a sorcery spell, so it is offered only with the stack empty --
// as a Room's other door is (mbc_room_alt_stack_lockstep_test.go). gorge's
// script search prices the suspension of Keldon Halberdier ({R}) only after
// the triggers the prefix's land taps queued have resolved; under the payer's
// own Manabarbs over an eleven-land board that pass walk is the 160-clone
// budget's tail, the search finds no script, and no cast or suspension is
// offered. The kernel priced the suspension with the triggers still waiting on
// the stack and offered it where gorge dropped the play as census_incomplete
// (tail-34-c001, seed 5699637453139540253, pair 112, decision 753).
func TestMBCKeldonHalberdierReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "suspend_special_action_waits_for_trigger_stack")
}
