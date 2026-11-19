package oracle

import "testing"

// Angel of Indemnity's ETB returns Pentarch Ward (an Aura) from the graveyard
// to the battlefield. As it enters its controller first chooses what to
// enchant, then answers the ward's own "as it enters, choose a color"
// replacement; only then does it enter and its "enter, draw a card" trigger
// go on the stack. At the choose-color ask the kernel had already moved the
// ward onto the battlefield attached to its enchant target and drawn its ETB
// trigger's card, while gorge still held the ward out of play with neither
// choice applied (decision 676 of 676; dr-mbc-abba-c000 pair 98, seed
// 163493, projection Cards[24].Pos).
func TestMBCPentarchWardReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "aura_return_enters_before_choices")
}
