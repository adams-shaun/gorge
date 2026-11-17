package payexec

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// plazaWheel is the stage-1 wheel Plaza of Heroes poses: {T}: Add {C}, the
// restricted "Add one mana of any color" (spendable on legendary spells only)
// and the reflected ability, whose label is a bare "Add " because it has no
// Produced$ (its colours are the legendary permanents' on the board).
func plazaWheel() *decision.Decision {
	return &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: []decision.Option{
		{Index: 0, Kind: "mana", Obj: 7, Ability: 0, Label: "Add C"},
		{Index: 1, Kind: "mana", Obj: 7, Ability: 1, Label: "Add any color"},
		{Index: 2, Kind: "mana", Obj: 7, Ability: 2, Label: "Add "},
	}}
}

// TestReflectedWheelOptionAnswersItsOwnWitnessStep: a witness step that taps
// Plaza's reflected ability (printed index 2) for {G} takes the bare-labelled
// option, not the restricted any-colour one the colour fits; a step that taps
// the any-colour ability (index 1) still takes it.
func TestReflectedWheelOptionAnswersItsOwnWitnessStep(t *testing.T) {
	green := decision.ManaAmount{4: 1}
	step := func(idx uint32) decision.PaymentActivation {
		return decision.PaymentActivation{Source: 7, Produces: green,
			Ability: decision.PaymentAbility{Kind: decision.PaymentAbilityPrinted, Index: idx}}
	}
	d := plazaWheel()
	if d.Options[1].Label == d.Options[2].Label {
		t.Fatal("precondition: the any-colour and reflected options are labelled alike")
	}
	if got, ok := matchMana(d, step(2)); !ok || len(got) != 1 || got[0] != 2 {
		t.Errorf("the reflected step picked %v (ok %v), want the bare option [2]", got, ok)
	}
	if got, ok := matchMana(d, step(1)); !ok || len(got) != 1 || got[0] != 1 {
		t.Errorf("the any-colour step picked %v (ok %v), want [1]", got, ok)
	}
}
