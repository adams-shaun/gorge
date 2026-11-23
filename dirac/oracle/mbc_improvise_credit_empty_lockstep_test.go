package oracle

import "testing"

// CR 702.126a, 601.2g-h: Archway of Innovation's emblem gives the next spell
// improvise, but Elvish Scout and Skyshroud Ranger ({G}) have no generic part
// for an artifact to pay, so the credit is empty and gorge plans the cast
// plainly: its prefix search funds Wooded Bastion's {G/W} filter fee from a
// Plains and offers the cast beside an untapped Forsaken Thresher. The kernel
// priced it "cost:credit" and withheld it (tail-15 pair 5, decision 865 of 865).
func TestMBCElvishScoutReprosLockstep(t *testing.T) {
	t.Parallel()
	stdReproNamed(t, "improvise_credit_empty")
}
