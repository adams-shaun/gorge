package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Mana that can't be spent to cast a nonartifact spell (CR 106.6: a Powerstone
// token's) cannot pay the mana part of an either-or additional cost on a
// creature spell, so the cast is not offered when that mana alone would fund
// the part.
func TestPotentialAltAddArtifactOnlyMana(t *testing.T) {
	t.Parallel()
	const (
		spellSrc = "Name:Alt Add Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nK:AlternateAdditionalCost:Discard<1/Card>:2\nOracle:x\n"
		landSrc  = "Name:Alt Add Mountain\nManaCost:no cost\nTypes:Basic Land Mountain\nOracle:x\n"
		stoneSrc = "Name:Powerstone Token\nManaCost:no cost\nColors:colorless\nTypes:Artifact Powerstone\n" +
			"A:AB$ Mana | Cost$ T | Produced$ C | RestrictValid$ CantCastNonArtifactSpells\nOracle:x\n"
	)
	const rockSrc = "Name:Alt Add Rock\nManaCost:no cost\nTypes:Artifact\nA:AB$ Mana | Cost$ T | Produced$ C\nOracle:x\n"
	plans := func(lands int, stone string) bool {
		e, _, spell := newFixtureDeck(t, 9412, spellSrc)
		for range lands {
			onBoard(t, e, 0, landSrc)
		}
		if stone != "" {
			onBoard(t, e, 0, stone)
		}
		e.G.Step = state.StepMain1
		ppPriority(t, e)
		parts := altAddCostParts(e.G.Obj(spell).Face())
		if len(parts) != 2 {
			t.Fatalf("alternate additional cost parsed %v, want two parts", parts)
		}
		o := decision.Option{Kind: "cast", Obj: spell}
		base := ParseCost(e.G.Obj(spell).Face().ManaCost)
		got := e.potentialAltAddPlan(0, o, base, parts)
		return got.Plan != nil && got.Reason == ""
	}
	if !plans(2, rockSrc) {
		t.Fatal("precondition: two Mountains and an unrestricted colourless rock do not fund {R}+{2}")
	}
	if plans(2, stoneSrc) {
		t.Fatal("the {2} part was planned through the Powerstone's artifact-only {C}")
	}
}

// The same barrier holds for restricted mana already floating (Hierophant's
// Chalice-style artifact-only {C}): two such units beside two Mountains do not
// fund {R}+{2}, and the cast is neither planned nor offered.
func TestPotentialAltAddFloatingArtifactOnlyMana(t *testing.T) {
	t.Parallel()
	const (
		spellSrc = "Name:Alt Add Bear\nManaCost:R\nTypes:Creature Bear\nPT:2/2\nK:AlternateAdditionalCost:Discard<1/Card>:2\nOracle:x\n"
		landSrc  = "Name:Alt Add Mountain\nManaCost:no cost\nTypes:Basic Land Mountain\nOracle:x\n"
	)
	e, _, spell := newFixtureDeck(t, 9413, spellSrc)
	onBoard(t, e, 0, landSrc)
	onBoard(t, e, 0, landSrc)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "ArtifactC", Amount: 2,
		Text: events.ManaRestrictionText("CantCastNonArtifactSpells", 0)})
	if pl := e.G.Players[0]; len(pl.RestrictedMana) == 0 || pl.Pool.Total() != 2 {
		t.Fatalf("precondition: pool %v restricted %v, want two restricted units", pl.Pool, pl.RestrictedMana)
	}
	e.G.Step = state.StepMain1
	d := ppPriority(t, e)
	parts := altAddCostParts(e.G.Obj(spell).Face())
	if len(parts) != 2 {
		t.Fatalf("alternate additional cost parsed %v, want two parts", parts)
	}
	got := e.potentialAltAddPlan(0, decision.Option{Kind: "cast", Obj: spell}, ParseCost("R"), parts)
	if got.Plan != nil && got.Reason == "" {
		t.Fatalf("the {2} part was planned through floating artifact-only mana: %+v", got.Plan)
	}
	for _, o := range d.Options {
		if o.Kind == "cast" && o.Obj == spell {
			t.Fatalf("the cast is offered on artifact-only mana: %+v", o)
		}
	}
}

// An either-or additional cost's mana part is paid from untapped sources, not
// from restricted floating mana that cannot pay it (pair 33 of the Daring
// Buccaneer focus run: three persistent artifact-only {C} floating beside a
// Mountain and two Forests). The witness must activate all three lands: a
// plan that taps the Mountain alone leans on the floating {C} for the {2}, and
// the cast it promises is not offered once the Mountain's {R} floats.
func TestAltAddPlanIgnoresFloatingArtifactOnlyMana(t *testing.T) {
	t.Parallel()
	const (
		spellSrc  = "Name:Alt Add Pirate\nManaCost:R\nTypes:Creature Human Pirate\nPT:2/2\nK:AlternateAdditionalCost:Reveal<1/Pirate>:2\nOracle:x\n"
		mountain  = "Name:Alt Add Mountain\nManaCost:no cost\nTypes:Basic Land Mountain\nOracle:x\n"
		forestSrc = "Name:Alt Add Forest\nManaCost:no cost\nTypes:Basic Land Forest\nOracle:x\n"
	)
	e, _, spell := newFixtureDeck(t, 9414, spellSrc)
	onBoard(t, e, 0, mountain)
	onBoard(t, e, 0, forestSrc)
	onBoard(t, e, 0, forestSrc)
	e.emit(events.Event{Kind: events.ManaAdd, Player: 0, Counter: "ArtifactC", Amount: 3,
		Text: events.ManaRestrictionText("CantCastNonArtifactSpells", 0)})
	if pl := e.G.Players[0]; len(pl.RestrictedMana) == 0 || pl.Pool.Total() != 3 {
		t.Fatalf("precondition: pool %v restricted %v, want three restricted units", pl.Pool, pl.RestrictedMana)
	}
	e.G.Step = state.StepMain1
	ppPriority(t, e)
	got := e.potentialModeCastPlan(0, decision.Option{Kind: "cast", Obj: spell})
	if got.Plan == nil || got.Reason != "" {
		t.Fatalf("precondition: no witness for {R}+{2} from three lands: %+v", got)
	}
	if n := len(got.Plan.Activations); n != 3 {
		t.Fatalf("the witness activates %d sources, want all three lands (the floating artifact-only {C} cannot pay the {2}): %+v", n, got.Plan.Activations)
	}
}
