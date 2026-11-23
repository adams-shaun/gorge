package oracle

import "testing"

const starfleetAcademyRepro = "paid_dynamic_mana_source_cast_offer"

// CR 601.2g, 605.3b: five Forests pay Subcommander T'Pol's {2}{U} through
// Starfleet Academy's "{5},{T}: Add X mana of one chosen colour" (X the
// creature types among non-Borg creatures you control). Gorge's prefix search
// exhausts its 160-node budget on the plain and the phased runs (the paid
// dynamic gap's fee branches over the Forests and an Orion land) and withholds
// the cast (decision 305 of std-trk pair 41). The kernel used to offer it: its
// freeBranchHopeless prune (#2670) skipped the dead free branches gorge walks.
// The prune now fails open beside a dynamic-amount filter, so both engines
// withhold the cast, the card is no longer refused (dirac/support/unrefused.tsv)
// and the recorded game replays in lockstep.
func TestMBCStarfleetAcademyReprosLockstep(t *testing.T) {
	t.Parallel()
	res := replayRawRepro(t, starfleetAcademyRepro)
	if res.Div != nil {
		t.Fatalf("%s's recorded game diverges at %d: %s", starfleetAcademyRepro, res.Div.At, res.Div)
	}
	if res.StallUnmodelled {
		t.Fatalf("%s's recorded game ends at an unmodelled stall: %q", starfleetAcademyRepro, res.Stall)
	}
}
