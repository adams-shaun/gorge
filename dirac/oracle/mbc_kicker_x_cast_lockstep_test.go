package oracle

import "testing"

const kickedFeeFilterRepro = "kicked_cast_funded_by_repeat_fee_filter"

// A kicked cast whose only red source is Signpost Scarecrow's "{2}: Add one
// mana of any colour", a repeatable fee-only mana ability (CR 605.3a, 602.2b):
// Kavu Aggressor's {2}{R} plus kicker {4} beside four Plains, a Blightstep
// Pathway and a Blighted Gorge (pool tail-45 pair 7, seed 761966, decision 224:
// gorge's options alone carry cast(#11{Kavu Aggressor})). Gorge's potential
// bound lifts the ability to unbounded and offers the kicked cast; the kernel's
// census prices a fee-only mana ability only in the trigger pay gate
// (mana_repeatpotential.go). That was the divergence the card was refused for.
// At the recorded decision both engines now offer the cast of Kavu Aggressor,
// and the recorded game replays clean on a table of its own cards, so the card
// is unrefused with this game as its proof (dirac/support/unrefused.tsv).
func TestMBCKavuAggressorReprosLockstep(t *testing.T) {
	t.Parallel()
	oneOffReproClean(t, kickedFeeFilterRepro)
}
