// Tests for snow-artifact mana: CR 107.4h makes snow a supertype, so mana
// from a permanent that is BOTH Snow and Artifact is snow mana regardless of
// its card type. A snow artifact's unit must land in Player.Snow (so a {S}
// pip can be paid from it) and in the artifact producer tally, and each
// Count$CastTotalManaSpent head counts the spent unit once.
package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/rules/pay"
	"github.com/adams-shaun/gorge/engine/state"
)

const snowArtifactFixture = "Name:Test Snow Rock\nTypes:Snow Artifact\nSVar:M:DB$ Mana | Produced$ C\nOracle:x\n"

// TestSnowArtifactManaIsSnow resolves the snow artifact's Mana ability and
// asserts its {C} unit is snow mana: the pool slot fills AND Player.Snow AND
// the artifact tally move with it through the one ManaAdd event.
func TestSnowArtifactManaIsSnow(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, snowArtifactFixture)
	rock := e.G.Obj(id)
	if rock == nil || rock.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source not on the battlefield: %+v", rock)
	}
	isSnow, isArtifact := false, false
	for _, ty := range rock.Face().Types {
		switch ty {
		case "Snow":
			isSnow = true
		case "Artifact":
			isArtifact = true
		}
	}
	if !isSnow || !isArtifact {
		t.Fatalf("precondition: source types %v, want Snow and Artifact", rock.Face().Types)
	}
	if got := e.G.Players[0].Snow[state.MC]; got != 0 {
		t.Fatalf("precondition: snow tally = %d, want 0 before the production", got)
	}
	if got := e.G.Players[0].TypedMana[state.TypedArtifact][state.MC]; got != 0 {
		t.Fatalf("precondition: artifact tally = %d, want 0 before the production", got)
	}

	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0},
		cards.ResolveSVar(rock.Face().SVars, "M"))

	if got := e.G.Players[0].Pool[state.MC]; got != 1 {
		t.Fatalf("pool {C} = %d, want 1", got)
	}
	if got := e.G.Players[0].Snow[state.MC]; got != 1 {
		t.Fatalf("snow tally {C} = %d, want 1: the snow artifact's mana is not snow", got)
	}
	if got := e.G.Players[0].TypedMana[state.TypedArtifact][state.MC]; got != 1 {
		t.Fatalf("artifact tally {C} = %d, want 1", got)
	}
}

// TestSnowArtifactManaPaysSnowCost produces the snow artifact's {C} and pays
// an {S} cost with it: the payment consumes the pool slot, the snow tally and
// the artifact tally together, once each.
func TestSnowArtifactManaPaysSnowCost(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, snowArtifactFixture)
	rock := e.G.Obj(id)
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0},
		cards.ResolveSVar(rock.Face().SVars, "M"))
	if got := e.G.Players[0].Snow[state.MC]; got != 1 {
		t.Fatalf("precondition: snow tally = %d, want 1 after the production", got)
	}
	if got := e.G.Players[0].Pool[state.MC]; got != 1 {
		t.Fatalf("precondition: pool {C} = %d, want 1 after the production", got)
	}

	cost := ParseCost("S")
	ok, _, _, spentSnow, spentTyped := pay.PayManaDescriptorForSpent(asPayer(e), 0,
		pay.Descriptor{Class: pay.PurposeSpell, Cost: &cost}, cost, nil, pay.PipRider{})
	if !ok {
		t.Fatal("the {S} cost was not payable from the snow artifact's mana")
	}
	if got := spentSnow[state.MC]; got != 1 {
		t.Fatalf("spent snow {C} = %d, want 1", got)
	}
	// The payment core returns the exclusive ManaUnits partition: the
	// composed unit's artifact provenance is its own slot
	// (state.TypedSnowArtifact), which the cast capture folds into the
	// artifact head.
	if got := spentTyped[state.TypedSnowArtifact][state.MC]; got != 1 {
		t.Fatalf("spent artifact {C} = %d, want 1", got)
	}
	if got := e.G.Players[0].Pool[state.MC]; got != 0 {
		t.Fatalf("pool {C} = %d after paying {S}, want 0", got)
	}
	if got := e.G.Players[0].Snow[state.MC]; got != 0 {
		t.Fatalf("snow tally {C} = %d after paying {S}, want 0", got)
	}
	if got := e.G.Players[0].TypedMana[state.TypedArtifact][state.MC]; got != 0 {
		t.Fatalf("artifact tally {C} = %d after paying {S}, want 0", got)
	}
}

// TestSnowArtifactManaPaysGenericFromComposedOnly pays a generic cost from a
// pool whose only {C} unit is the composed snow-artifact one: the exclusive
// payment partition holds it in state.TypedSnowArtifact with an empty
// artifact-only slot, and the plain-unit search must still consume it from
// the pool once, moving all three tallies with it.
func TestSnowArtifactManaPaysGenericFromComposedOnly(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, snowArtifactFixture)
	rock := e.G.Obj(id)
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0},
		cards.ResolveSVar(rock.Face().SVars, "M"))
	if got := e.G.Players[0].SnowTyped[state.MC]; got != 1 {
		t.Fatalf("precondition: composed tally = %d, want 1 after the production", got)
	}
	units := e.G.Players[0].ManaUnits()
	if units[state.TypedSnowArtifact][state.MC] != 1 || units[state.TypedArtifact][state.MC] != 0 {
		t.Fatalf("precondition: exclusive partition artifact=%d composed=%d, want 0/1",
			units[state.TypedArtifact][state.MC], units[state.TypedSnowArtifact][state.MC])
	}

	cost := ParseCost("1")
	ok, _, _, _, _ := pay.PayManaDescriptorForSpent(asPayer(e), 0,
		pay.Descriptor{Class: pay.PurposeOther, Cost: &cost}, cost, nil, pay.PipRider{})
	if !ok {
		t.Fatal("the generic cost was not payable from the composed unit")
	}
	if got := e.G.Players[0].Pool[state.MC]; got != 0 {
		t.Fatalf("pool {C} = %d after paying {1}, want 0", got)
	}
	if got := e.G.Players[0].Snow[state.MC]; got != 0 {
		t.Fatalf("snow tally {C} = %d after paying {1}, want 0", got)
	}
	if got := e.G.Players[0].TypedMana[state.TypedArtifact][state.MC]; got != 0 {
		t.Fatalf("artifact tally {C} = %d after paying {1}, want 0", got)
	}
	if got := e.G.Players[0].SnowTyped[state.MC]; got != 0 {
		t.Fatalf("composed tally {C} = %d after paying {1}, want 0", got)
	}
}
