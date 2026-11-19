package oracle

import "testing"

// CR 602.2b, 106.7: Immersturm Skullcairn's "{1}{B}{R}{R}, {T}, Sacrifice"
// beside an Incubation Druid is payable: the Druid reflects the Skullcairn's
// own {B} while the Skullcairn taps for the cost, and a Forest and three
// Mountains pay the rest. gorge's potential walk lists the play (its bound
// counts the Skullcairn's own mana) and the prefix search taps the Druid; the
// kernel's gaps walk had removed the Skullcairn's mana and never offered it
// (tail-55 autopay pair 174, decision 595 of 595).
func TestMBCImmersturmSkullcairnReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "own_tapped_source_walk_candidate_beside_reflected_druid")
}
