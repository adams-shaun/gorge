package payexec

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// anyDifferentAsk is the ask a Combo AnyDifferent production (Firemind Vessel) poses: two
// units, each listing the five colours, one exclusion group per colour.
func anyDifferentAsk() *decision.Decision {
	d := &decision.Decision{Kind: decision.KChoose, Min: 2, Max: 2}
	for unit := 0; unit < 2; unit++ {
		for _, c := range []string{"W", "U", "B", "R", "G"} {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana",
				Label: "Add " + c, ManaSymbol: c, Group: "manacolour:" + c})
		}
	}
	return d
}

// TestWindowChoicesDistinctColours: a window answer to the AnyDifferent ask names two
// different colours, so Decision.Validate accepts it; the per-slot pick alone would name the
// owed colour twice.
func TestWindowChoicesDistinctColours(t *testing.T) {
	d := anyDifferentAsk()
	owed := decision.ManaAmount{}
	owed[0] = 2 // {W}{W} owed: slot by slot the owed colour is W both times
	got := WindowChoices(d, owed)
	if len(got) != 2 {
		t.Fatalf("answer %v, want two choices", got)
	}
	// precondition: the same picks without the exclusion repeat the colour
	for slot := 0; slot < 2; slot++ {
		one := decision.Decision{Options: d.Options[slot*5 : slot*5+5]}
		if i := WindowAnswer(&one, owed); d.Options[i].ManaSymbol != "W" {
			t.Fatalf("slot %d unconstrained pick %d is %s, want the owed W", slot, i, d.Options[i].ManaSymbol)
		}
	}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Choices: got}); err != nil {
		t.Fatalf("answer %v refused: %v", got, err)
	}
	if d.Options[got[0]].ManaSymbol != "W" || d.Options[got[1]].ManaSymbol == "W" {
		t.Fatalf("answer %v: want the owed W first, then a different colour", got)
	}
}
