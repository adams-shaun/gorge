package oracle

import "testing"

// CR 601.2c with CR 702.16b: Sword of Once and Future's trigger offers a free
// cast of an instant or sorcery with mana value 2 or less from the graveyard,
// and a spell with no legal target isn't offered. The equipped Zephyr Falcon
// (protection from black) is the only nonblack creature, so Ghastly Demise,
// being black, can't target it. The kernel priced the candidate's targets
// against the resolving trigger's colourless source instead of the spell's own
// colours and offered it as a third mode (decision 631 of tail-68-c002 pair 3).
func TestMBCHowlingFuryReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "free_cast_candidate_colour_protection")
}
