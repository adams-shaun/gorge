package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/state"
)

// TestExchangeTextBoxUsesCurrentDerivedText is the regression for the CR 612
// exchange rule: api:ExchangeTextBox exchanges the objects' text boxes AS THEY
// READ at resolution, so an object already carrying a ChangeText substitution
// contributes its SUBSTITUTED text, not its printed Oracle. Non-vacuous
// preconditions: Alpha's text really was substituted before the exchange
// (so the assertion distinguishes derived text from printed text), and the
// two objects' texts differ before the exchange.
func TestExchangeTextBoxUsesCurrentDerivedText(t *testing.T) {
	t.Parallel()
	a := card(t, "Name:Alpha\nManaCost:B\nTypes:Creature Human\nPT:1/1\nOracle:Protection from red.\n")
	b := card(t, "Name:Beta\nManaCost:B\nTypes:Creature Beast\nPT:2/2\nOracle:Trample.\n")
	e, _, _ := corpusDeckEngine(t, nil, []*cards.Card{a, b})
	var aID, bID state.ObjID
	for i := range e.G.Objs {
		switch e.G.Objs[i].Card {
		case a:
			aID = e.G.Objs[i].ID
		case b:
			bID = e.G.Objs[i].ID
		}
	}
	if aID == 0 || bID == 0 || aID == bID {
		t.Fatalf("setup: a=%d b=%d, both must be distinct battlefield objects", aID, bID)
	}

	// Step 1: substitute Alpha's text (red -> black) through the real
	// ChangeText primitive, so Alpha's current text is its DERIVED text.
	changeSA := &cards.SA{API: "ChangeText", Params: map[string]string{
		"ChangeColorWord": "red black", "Duration": "Permanent", "Defined": "Targeted"}}
	changeCtx := &effects.Ctx{Source: aID, Controller: 0, Targets: []state.Target{{Obj: aID}}}
	effects.Resolve(e, changeCtx, changeSA)

	// Precondition: the substitution took effect and differs from the printed
	// Oracle, so the exchange assertion below cannot pass on printed text.
	if got := e.Text(aID); got != "Protection from black." {
		t.Fatalf("precondition: Alpha text after ChangeText = %q, want the substituted %q (printed is %q)",
			got, "Protection from black.", a.Faces[0].Oracle)
	}
	if e.Text(bID) != "Trample." {
		t.Fatalf("precondition: Beta text = %q, want printed %q", e.Text(bID), "Trample.")
	}
	if e.Text(aID) == e.Text(bID) {
		t.Fatalf("precondition: the two texts must differ before exchange, both %q", e.Text(aID))
	}

	// Step 2: exchange the two text boxes.
	sa := &cards.SA{API: "ExchangeTextBox", Params: map[string]string{"Duration": "AsLongAsInPlay", "Defined": "Targeted"}}
	ctx := &effects.Ctx{Source: aID, Controller: 0, Targets: []state.Target{{Obj: aID}, {Obj: bID}}}
	effects.Resolve(e, ctx, sa)

	if got := e.Text(aID); got != "Trample." {
		t.Fatalf("Alpha after exchange = %q, want Beta's current text %q", got, "Trample.")
	}
	if got := e.Text(bID); got != "Protection from black." {
		t.Fatalf("Beta after exchange = %q, want Alpha's CURRENT (substituted) text %q; "+
			"printed would be %q", got, "Protection from black.", "Protection from red.")
	}
}
