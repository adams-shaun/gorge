package oracle

import "testing"

// CR 601.2g with CR 605.1a: Lotus Blossom's "{T}, Sacrifice this artifact: Add
// X mana of any one colour, where X is its petal counters" funds God-Eternal
// Rhonas's {3}{G}{G} from two Forests, two Islands and a charged Lotus
// Blossom. gorge's potential planner activates the source while paying, so it
// offers the cast; the kernel left the dynamic {T}, sacrifice source out of a
// cast offer's census even when the potential walk prices the play (decision
// 378, tail-74 pair 6).
func TestMBCGodEternalRhonasReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "dynamic_sacrifice_mana_cast_offer")
}
