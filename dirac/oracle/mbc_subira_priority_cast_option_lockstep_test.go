package oracle

import "testing"

// CR 601.2a-e, 601.3 and 601.6: a may-play cast's permission (The Reality
// Chip's "As long as The Reality Chip is attached to a creature, you may play
// lands and cast spells from the top of your library") is read when the spell
// is proposed, before any mana ability is activated (601.2g), so a payment
// that consumes the permanent granting it does not unmake the proposal. With
// only two untapped Islands and Goldhound ({T}, Sacrifice Goldhound: Add one
// mana of any color) carrying the Chip, Subira, Tulzidi Caravanner ({2}{R}, the
// top card of the library) is castable exactly by sacrificing its own
// permission's bearer: gorge's payment planner once priced no such witness
// (the witness replay's end menu re-reads the permission and refuses) and the
// autopay surface withheld the cast; the kernel offered it. tail-47-c001 pair
// 1 parts at "options options" (decision 176).
func TestMBCSubiraTulzidiCaravannerReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "subira_priority_cast_option")
}
