package oracle

import "testing"

// At p1's priority (tail-76 pair 0, decision 677) the kernel offered Stensia
// Bloodhall's {3}{B}{R},{T} ping while gorge offered only pass. The plan is
// legal per CR 605.3: the two islands pay the {1} fees of Painted Bluffs's and
// Cave of Temptation's any-colour abilities, which pay {B}/{R}, and the
// remaining islands plus the forest pay the generic {3}. The recording replays
// on its own decks' table.
func TestMBCStensiaBloodhallReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "stensia_bloodhall_activation_offer")
}
