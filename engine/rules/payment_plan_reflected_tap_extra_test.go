package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// A static TapsForMana trigger adding one mana of the type the tap produced
// (Lavaleaper's DB$ ManaReflected | ColorOrType$ Type | ReflectProperty$
// Produced) folds into a planned tap of a source making one type, as a
// literal extra does (CR 106.12a, 605.1b): the Mountain's plan step makes
// {R}{R}, so one Mountain pays a {1}{R} spell. A source making two types
// leaves the extra a choice and stays deferred.
const reflectedTapExtraCarrier = "Name:Reflect Test\nTypes:Creature Elemental\nPT:4/4\n" +
	"T:Mode$ TapsForMana | ValidCard$ Land | Execute$ OnMana | TriggerZones$ Battlefield | Static$ True | TriggerDescription$ x\n" +
	"SVar:OnMana:DB$ ManaReflected | ColorOrType$ Type | ReflectProperty$ Produced | Defined$ TriggeredActivator\nOracle:x\n"

func TestPaymentPlanReflectedTapExtraFolds(t *testing.T) {
	const twoSpell = "Name:Reflect Plan\nManaCost:1 R\nTypes:Instant\nA:SP$ Draw | Num$ 1\nOracle:x\n"
	t.Run("one type folds", func(t *testing.T) {
		e, _, spell := newFixtureDeck(t, 9361, twoSpell)
		mtn := onBoard(t, e, 0, ppMountain)
		onBoard(t, e, 1, reflectedTapExtraCarrier)
		a := ppAction(t, ppAsk(t, e), spell)
		acts := a.Plans[0].Activations
		if len(acts) != 1 || acts[0].Source != mtn {
			t.Fatalf("plan = %#v, want the one Mountain", acts)
		}
		if got := acts[0].Produces[state.ManaIndex('R')]; got != 2 {
			t.Fatalf("the Mountain's step produces %v, want {R}{R}", acts[0].Produces)
		}
	})
	t.Run("two types stay deferred", func(t *testing.T) {
		e, _, _ := newFixtureDeck(t, 9362, twoSpell)
		dual := onBoard(t, e, 0, "Name:Dual Test\nTypes:Land\nA:AB$ Mana | Cost$ T | Produced$ R G\nOracle:x\n")
		onBoard(t, e, 1, reflectedTapExtraCarrier)
		for _, a := range ppAsk(t, e).PaymentActions {
			for _, p := range a.Plans {
				for _, act := range p.Activations {
					if act.Source == dual {
						t.Fatalf("a two-type source scheduled beside a reflected extra: %#v", p.Activations)
					}
				}
			}
		}
	})
}
