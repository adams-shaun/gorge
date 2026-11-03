package manabrew

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/view"
	mb "github.com/adams-shaun/gorge/server/protocol/manabrew"
)

func colourAsk() *decision.Decision {
	d := &decision.Decision{Seq: 9, Kind: decision.KChoose, Prompt: "Add 3 mana in any combination of colors", Min: 3, Max: 3}
	for range 3 {
		for _, colour := range []string{"W", "U"} {
			d.Options = append(d.Options, decision.Option{Index: len(d.Options), Kind: "mana", Obj: 20, ManaSymbol: colour, Label: "Add " + colour})
		}
	}
	return d
}

// An ask that offers a colour once per unit lets the colour repeat, and three of
// one colour answers three distinct options.
func TestChooseColorRepeatsAColourOfferedPerUnit(t *testing.T) {
	d := colourAsk()
	tr := New("t", 1, nil)
	msg, _, err := tr.promptChoose(d, &view.View{})
	if err != nil {
		t.Fatal(err)
	}
	in := msg.Input.Value.(mb.ChooseColorInput)
	if !in.RepeatAllowed {
		t.Fatal("repeatAllowed is false on an ask offering each colour three times")
	}
	out := tr.parseChooseColor(mb.ColorDecision{ChosenColors: map[string]int{"U": 2, "W": 1}}, &Pending{Prompt: msg, Decision: d})
	if out.Err != nil {
		t.Fatalf("a legal combination was refused: %+v", out.Err)
	}
	if got := out.Intent.Choices; len(got) != 3 {
		t.Fatalf("choices = %v", got)
	}
}

// One option per colour keeps repeatAllowed false, as the ask can't take two.
func TestChooseColorDoesNotRepeatAColourOfferedOnce(t *testing.T) {
	d := &decision.Decision{Seq: 9, Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "mana", ManaSymbol: "W"}, {Index: 1, Kind: "mana", ManaSymbol: "U"}}}
	msg, _, err := New("t", 1, nil).promptChoose(d, &view.View{})
	if err != nil {
		t.Fatal(err)
	}
	if msg.Input.Value.(mb.ChooseColorInput).RepeatAllowed {
		t.Fatal("repeatAllowed is true on an ask offering each colour once")
	}
}
