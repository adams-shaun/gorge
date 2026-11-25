package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Component Pouch's "{T}, Remove a component counter from it: Add two mana of
// different colors" is Produced$ Combo AnyDifferent beside a counter cost: the
// controller picks two different colours, the ability adds one of each and the
// counter is gone (CR 106.1, 605.1a, 602.2b).
func TestComponentPouchAddsTwoDifferentColours(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	source := onBoard(t, e, 0, "Name:Component Pouch\nTypes:Artifact\n"+
		"A:AB$ Mana | Cost$ T SubCounter<1/COMPONENT> | Amount$ 2 | Produced$ Combo AnyDifferent\nOracle:x\n")
	o := e.G.Obj(source)
	if o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("Component Pouch precondition failed: %+v", o)
	}
	e.G.Obj(source).AddCounter("COMPONENT", 1)
	if got := e.G.Obj(source).Counter("COMPONENT"); got != 1 {
		t.Fatalf("component counters = %d, want 1 before the activation", got)
	}

	e.askPriority(0)
	submitChoices(t, e, activateOption(t, e, source))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 2 || d.Max != 2 || len(d.Options) != 10 {
		t.Fatalf("Component Pouch ask = %+v, want two choices from ten unit-colour options", d)
	}
	var white, red int
	for _, opt := range d.Options {
		switch opt.ManaSymbol {
		case "W":
			white = opt.Index
		case "R":
			red = opt.Index
		}
	}
	submitChoices(t, e, white, red)
	if got, want := e.G.Players[0].Pool, (state.Mana{state.MW: 1, state.MR: 1}); got != want {
		t.Fatalf("Component Pouch pool = %v, want %v", got, want)
	}
	if got := e.G.Obj(source).Counter("COMPONENT"); got != 0 {
		t.Fatalf("component counters = %d, want 0 after the activation", got)
	}
	if !e.G.Obj(source).Tapped {
		t.Fatal("Component Pouch stayed untapped after its activation")
	}
}

// The potential bound prices the Pouch as one of each colour while it holds a
// component counter and as nothing once the counter is gone: the cost is
// payable only with the counter, so the ability is neither unbounded nor
// offered without it.
func TestComponentPouchPotentialNeedsTheCounter(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	source := onBoard(t, e, 0, "Name:Component Pouch\nTypes:Artifact\n"+
		"A:AB$ Mana | Cost$ T SubCounter<1/COMPONENT> | Amount$ 2 | Produced$ Combo AnyDifferent\nOracle:x\n")
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield || o.Tapped || o.Counter("COMPONENT") != 0 {
		t.Fatalf("Component Pouch precondition failed: %+v", o)
	}
	if got := e.PotentialMana(0); got != (state.Mana{}) {
		t.Fatalf("counterless Pouch potential = %v, want none", got)
	}
	e.G.Obj(source).AddCounter("COMPONENT", 1)
	if got, want := e.PotentialMana(0), (state.Mana{state.MW: 1, state.MU: 1, state.MB: 1, state.MR: 1, state.MG: 1}); got != want {
		t.Fatalf("Pouch with a counter potential = %v, want the bounded %v", got, want)
	}
}
