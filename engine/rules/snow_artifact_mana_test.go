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
	"github.com/adams-shaun/gorge/engine/events"
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

// TestSnowArtifactManaCountsOnBothCastSpendHeads is the end-to-end cast: the
// real snow artifact produces its composed unit, the cast pays {C} from it,
// and BOTH pay-time heads capture the one spent unit exactly once -- the snow
// head (Count$CastTotalManaSpent Snow's provenance, here read by the ETB's
// draw) and the artifact head (the composed slot folded into
// manaSpentArtifact beside the exclusive artifact slot).
func TestSnowArtifactManaCountsOnBothCastSpendHeads(t *testing.T) {
	reader := "Name:Test Snow Reader\nManaCost:C\nTypes:Creature\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigDraw | TriggerDescription$x\n" +
		"SVar:TrigDraw:DB$ Draw | NumCards$ SpentSnow\n" +
		"SVar:SpentSnow:Count$CastTotalManaSpent Snow\nOracle:x\n"
	e := handEngineTokens(t, card(t, reader))
	rockID := onBoardCard(t, e, 0, card(t, snowArtifactFixture))
	rock := e.G.Obj(rockID)
	effects.Resolve(e, &effects.Ctx{Source: rockID, Controller: 0},
		cards.ResolveSVar(rock.Face().SVars, "M"))
	if got := e.G.Players[0].SnowTyped[state.MC]; got != 1 {
		t.Fatalf("precondition: composed tally = %d, want 1 after the production", got)
	}
	if got := e.G.Players[0].Snow[state.MC]; got != 1 {
		t.Fatalf("precondition: snow tally = %d, want 1 after the production", got)
	}

	spell := e.G.Zone(state.ZHand, 0)[0]
	castMode(t, e, spell, "")
	finishCast(t, e, spell)
	drainEtb(t, e)
	o := e.G.Obj(spell)
	if o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("the reader did not resolve to the battlefield: %+v", o)
	}
	if got := o.ManaSpent; got != 1 {
		t.Fatalf("ManaSpent = %d, want 1", got)
	}
	if got := o.ManaSnowSpent; got != 1 {
		t.Fatalf("ManaSnowSpent = %d, want 1 (the snow head counts the composed unit once, as snow)", got)
	}
	if got := o.ManaArtifactSpent; got != 1 {
		t.Fatalf("ManaArtifactSpent = %d, want 1 (the artifact head counts the composed unit once, as artifact)", got)
	}
	// The snow head read the capture for real: one draw.
	if got := len(e.G.Zone(state.ZHand, 0)); got != 1 {
		t.Fatalf("hand holds %d cards, want 1 (the ETB drew Count$CastTotalManaSpent Snow = 1)", got)
	}
	// The spend moved all four tallies of the composed unit together.
	if got := e.G.Players[0].Pool[state.MC]; got != 0 {
		t.Fatalf("pool {C} = %d after the cast, want 0", got)
	}
	if got := e.G.Players[0].Snow[state.MC]; got != 0 {
		t.Fatalf("snow tally {C} = %d after the cast, want 0", got)
	}
	if got := e.G.Players[0].TypedMana[state.TypedArtifact][state.MC]; got != 0 {
		t.Fatalf("artifact tally {C} = %d after the cast, want 0", got)
	}
	if got := e.G.Players[0].SnowTyped[state.MC]; got != 0 {
		t.Fatalf("composed tally {C} = %d after the cast, want 0", got)
	}
}

// TestSnowArtifactComposedUnitDrainsAtManaClear pins the composed class in
// the ManaClear drain: a floating composed unit leaves the pool, its parent
// artifact tally and its snow tally together, and the overlap clamps hold
// (no tally outlives the pool it partitions).
func TestSnowArtifactComposedUnitDrainsAtManaClear(t *testing.T) {
	e := layerEngine(t)
	id := onBoard(t, e, 0, snowArtifactFixture)
	rock := e.G.Obj(id)
	effects.Resolve(e, &effects.Ctx{Source: id, Controller: 0},
		cards.ResolveSVar(rock.Face().SVars, "M"))
	p := e.G.Players[0]
	if p.Pool[state.MC] != 1 || p.Snow[state.MC] != 1 ||
		p.TypedMana[state.TypedArtifact][state.MC] != 1 || p.SnowTyped[state.MC] != 1 {
		t.Fatalf("precondition: pool/snow/artifact/composed = %d/%d/%d/%d, want 1/1/1/1",
			p.Pool[state.MC], p.Snow[state.MC], p.TypedMana[state.TypedArtifact][state.MC], p.SnowTyped[state.MC])
	}

	e.emit(events.Event{Kind: events.ManaClear, Player: 0})
	p = e.G.Players[0]
	if got := p.Pool[state.MC]; got != 0 {
		t.Fatalf("pool {C} = %d after ManaClear, want 0", got)
	}
	if got := p.Snow[state.MC]; got != 0 {
		t.Fatalf("snow tally {C} = %d after ManaClear, want 0", got)
	}
	if got := p.TypedMana[state.TypedArtifact][state.MC]; got != 0 {
		t.Fatalf("artifact tally {C} = %d after ManaClear, want 0", got)
	}
	if got := p.SnowTyped[state.MC]; got != 0 {
		t.Fatalf("composed tally {C} = %d after ManaClear, want 0", got)
	}
}

// TestSnowArtifactWindowTapIsUndoable pins the window-tap reversal over a
// composed unit: PlainOrSnowManaCounter now admits the composed counter (so a
// snow-artifact activation is ManaUndo-reversible where an exclusive
// artifact one is not), the undoability predicate holds over the composed
// tallies, and the reversal drives the pool, both parent tallies and the
// overlap back to zero and untaps the source.
func TestSnowArtifactWindowTapIsUndoable(t *testing.T) {
	e := layerEngine(t)
	rockID := onBoard(t, e, 0, snowArtifactFixture)
	rock := e.G.Obj(rockID)
	// The admission is the record path's verdict (CloseWindowTap sets
	// Reversible from it): the composed form is admitted, the exclusive
	// artifact form is not.
	if !pay.PlainOrSnowManaCounter("SArtifactC") {
		t.Fatal("PlainOrSnowManaCounter refuses the composed counter: a snow-artifact activation would be recorded irreversible")
	}
	if pay.PlainOrSnowManaCounter("ArtifactC") {
		t.Fatal("PlainOrSnowManaCounter admits the artifact counter; only one ManaUndo cannot reverse a typed unit")
	}
	// The activation's add: one composed unit, and the activation tapped the
	// source. effects.Resolve produces the same ManaAdd the window recorded.
	effects.Resolve(e, &effects.Ctx{Source: rockID, Controller: 0},
		cards.ResolveSVar(rock.Face().SVars, "M"))
	e.emit(events.Event{Kind: events.Tap, Obj: rockID})
	p := e.G.Players[0]
	if p.Pool[state.MC] != 1 || p.Snow[state.MC] != 1 || p.SnowTyped[state.MC] != 1 {
		t.Fatalf("precondition: pool/snow/composed = %d/%d/%d, want 1/1/1",
			p.Pool[state.MC], p.Snow[state.MC], p.SnowTyped[state.MC])
	}
	if !rock.Tapped {
		t.Fatal("precondition: the source is not tapped")
	}
	// The record as CloseWindowTap leaves it: closed and judged reversible,
	// with the exact ManaAdd the activation made.
	cp := &pay.CastPayment{WindowTaps: []pay.WindowTap{{
		Source: rockID, Closed: true, Reversible: true,
		Adds: []pay.WindowTapAdd{{Counter: "SArtifactC", Amount: 1}},
	}}}
	if _, ok := pay.UndoableWindowTap(asPayer(e), cp, 0); !ok {
		t.Fatal("the snow-artifact window tap is not undoable (the composed tallies are not admitted by the predicate)")
	}
	pay.UndoWindowTap(asPayer(e), cp, 0)
	p = e.G.Players[0]
	if got := p.Pool[state.MC]; got != 0 {
		t.Fatalf("pool {C} = %d after the undo, want 0", got)
	}
	if got := p.Snow[state.MC]; got != 0 {
		t.Fatalf("snow tally {C} = %d after the undo, want 0", got)
	}
	if got := p.TypedMana[state.TypedArtifact][state.MC]; got != 0 {
		t.Fatalf("artifact tally {C} = %d after the undo, want 0", got)
	}
	if got := p.SnowTyped[state.MC]; got != 0 {
		t.Fatalf("composed tally {C} = %d after the undo, want 0", got)
	}
	if e.G.Obj(rockID).Tapped {
		t.Fatal("the source stayed tapped after the undo")
	}
}
