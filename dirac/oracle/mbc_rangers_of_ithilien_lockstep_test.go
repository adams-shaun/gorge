package oracle

import "testing"

// CR 601.2g with CR 605.3a: Rangers of Ithilien ({2}{U}{U}) is castable from a
// board whose blue comes partly from Vivid Creek, whose "{T}, remove a charge
// counter: Add one mana of any colour" is a census gap beside its plain
// "{T}: Add {U}". The oracle's "choose a mana ability" ask flattens the
// any-colour ability into one option per colour, so those colours are answers
// of the ability ask, not a second colour ask. Dirac charged a node for the
// second ask and budget-limited the filter search that funds Viridescent Bog's
// {1}, dropping the cast (decision 1013 of 1013, tail-12 pair 16 seed 438272).
func TestMBCRangersOfIthilienReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "rangers_of_ithilien")
}
