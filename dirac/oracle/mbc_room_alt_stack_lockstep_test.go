package oracle

import "testing"

// CR 307.1, 117.1a with CR 605.3b: a Room's other door is an enchantment cast
// at sorcery speed, so gorge's script search prices it only once the triggers
// the prefix's land taps queued have resolved. Under the player's own
// Manabarbs at 5 life that is ten damage, a line that ends the game: no script,
// and neither door is offered. The kernel treated the door as an instant-speed
// play and offered it with the triggers still waiting on the stack
// (tail-34-c000, seed 179688, decision 1071).
func TestMBCSpikedCorridorReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "room_alt_cast_waits_for_trigger_stack")
}
