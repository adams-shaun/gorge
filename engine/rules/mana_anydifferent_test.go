package rules

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// Firemind Vessel's "{T}: Add two mana of different colors" (Produced$ Combo
// AnyDifferent) is a bounded choice of two DIFFERENT colours: the controller
// picks two of WUBRG, the ability adds one of each, and the potential bound
// prices one of each colour rather than the old open every-colour sentinel
// (CR 106.1).
func TestComboAnyDifferentProducesTwoColours(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	source := onBoard(t, e, 0, "Name:Firemind Vessel\nTypes:Artifact\n"+
		"A:AB$ Mana | Cost$ T | Amount$ 2 | Produced$ Combo AnyDifferent\nOracle:x\n")
	if o := e.G.Obj(source); o == nil || o.Zone != state.ZBattlefield || o.Tapped {
		t.Fatalf("AnyDifferent source precondition failed: %+v", o)
	}

	// The potential bound is finite: one of each colour, no unbounded
	// sentinel and no phantom colourless.
	if got, want := e.PotentialMana(0), (state.Mana{state.MW: 1, state.MU: 1, state.MB: 1, state.MR: 1, state.MG: 1}); got != want {
		t.Fatalf("AnyDifferent potential = %v, want the bounded %v", got, want)
	}

	e.askPriority(0)
	submitChoices(t, e, activateOption(t, e, source))
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose || d.Min != 2 || d.Max != 2 || len(d.Options) != 10 {
		t.Fatalf("AnyDifferent ask = %+v, want two choices from ten unit-colour options", d)
	}
	var whites, reds []int
	for _, o := range d.Options {
		if o.Kind != "mana" || o.Obj != source {
			t.Fatalf("AnyDifferent option %+v is not a mana option for %d", o, source)
		}
		switch o.ManaSymbol {
		case "W":
			whites = append(whites, o.Index)
		case "R":
			reds = append(reds, o.Index)
		}
	}
	if len(whites) != 2 || len(reds) != 2 {
		t.Fatalf("AnyDifferent offered W=%v R=%v, want two units of each", whites, reds)
	}
	// One home for the legal-answer rule: Validate rejects choosing one colour
	// twice, and the bot's first-Min answer (the deterministic KChoose fill)
	// is accepted by the same validator.
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{whites[0], whites[1]}}); err == nil ||
		!strings.Contains(err.Error(), "mutually exclusive") {
		t.Fatalf("same-colour answer err = %v, want a mutual-exclusion rejection", err)
	}
	bot := []int{d.Options[0].Index, d.Options[1].Index}
	if err := d.Validate(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: bot}); err != nil {
		t.Fatalf("bot's first-Min answer %v rejected: %v", bot, err)
	}
	submitChoices(t, e, whites[0], reds[0])
	pool := e.G.Players[0].Pool
	want := state.Mana{state.MW: 1, state.MR: 1}
	if pool != want {
		t.Fatalf("AnyDifferent pool = %v, want %v", pool, want)
	}
	if !e.G.Obj(source).Tapped {
		t.Fatal("AnyDifferent source stayed untapped after its activation")
	}
}
