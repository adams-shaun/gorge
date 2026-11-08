package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRestrictedAmountWitnessX (gorge; CR 106.6, 601.2b): the largest-X
// witness of an {X} spell counts a restricted evaluated amount (Helga's
// power) in the total it divides by X, as gorge's census does. Mockingbird
// {X}{U} beside Helga (two mana of one colour) and Plains, Plains, Forest is
// X = 4 (five mana), not the lands' X = 3: the witness taps every source.
func TestRestrictedAmountWitnessX(t *testing.T) {
	var cost kop.ManaCost
	cost.X = 1
	cost.Pips[kop.U] = 1
	var av avail
	av.addUnits(1<<kop.W, 2)
	av.addUnits(1<<kop.G, 1)
	if got := av.maxX(&cost, pipMaskOf(&cost)); got != -1 {
		t.Fatalf("the lands alone price X = %d, want none (no blue)", got) // the precondition: Helga's blue is what pays {U}
	}
	if got := restrWitnessX(3, av, []uint8{2}, &cost); got != 4 {
		t.Errorf("witness X beside Helga's two mana = %d, want 4", got)
	}
}
