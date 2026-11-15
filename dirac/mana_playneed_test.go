package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestPlayNeedReadsThePrintedBaseWhenTheGrantSearchDeclines: the filter
// search prices plays the grant search declines (a spell's own RaiseCost
// Waterbend here). playNeed must still report the printed pips: gorge's
// scriptNeedColours reads the printed base for every Mode "" cast, and a
// need set gated on grantPlayCost's ok would be empty, leaving the filter
// colour ask in engine order.
func TestPlayNeedReadsThePrintedBaseWhenTheGrantSearchDeclines(t *testing.T) {
	tbl, id := wbTable(t, "Benevolent River Spirit", "Island")
	sc := newScenTableLib(t, tbl, id["Island"], Config{NoElide: true})
	spirit := sc.hand(id["Benevolent River Spirit"], 0)
	a := MakeAction(AkCast, TCode(spirit), 0, 0)
	if _, _, _, ok := sc.s.grantPlayCost(sc.k, 0, a, false); ok {
		t.Fatal("precondition: the grant search prices the waterbend cast, so this is not the declined shape")
	}
	need := sc.s.playNeed(sc.k, a)
	if !need[kop.U] {
		t.Fatalf("playNeed %v lacks U for the {U}{U} waterbend cast the grant search declines", need)
	}
}
