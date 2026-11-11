package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules/pay"
)

// xColorAbilitySA parses a card whose activated ability pays {X} with black
// only (XColor$ Black, Crypt Rats' shape) and returns that ability.
func xColorAbilitySA(t *testing.T) *cards.SA {
	t.Helper()
	const src = "Name:XColor Slot Probe\n" +
		"ManaCost:2 B\n" +
		"Types:Creature\n" +
		"PT:1/1\n" +
		"A:AB$ DamageAll | Cost$ X | XColor$ B | NumDmg$ X | ValidCards$ Creature | SpellDescription$ x\n" +
		"Oracle:x\n"
	c, diags := cards.ParseBytes("xcolor_slot_probe.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse diags: %+v", diags)
	}
	for _, sa := range c.Faces[0].Abilities {
		if sa.Kind == "AB" {
			if sa.CostSlot() == nil {
				t.Fatal("precondition: the ability has no cost slot (the first-writer race needs one)")
			}
			return sa
		}
	}
	t.Fatal("precondition: no AB$ ability parsed")
	return nil
}

// TestAbilityXColorSurvivesCostOfFirst: AbilityCostOf and the engine's
// AbilityCompiledCost share the ability's first-writer-wins cost slot, so
// whichever runs first decides the entry for the process. A read through
// AbilityCostOf first (the loyalty classifier, stackXAnnounced) must still
// leave the black XColor mask on the engine path's cost.
func TestAbilityXColorSurvivesCostOfFirst(t *testing.T) {
	sa := xColorAbilitySA(t)
	first := pay.AbilityCostOf(sa)
	if first.Cost.X != 1 {
		t.Fatalf("precondition: Cost$ X compiled to %+v, want one {X}", first.Cost)
	}
	e := New(Config{Seed: 1, Names: []string{"a", "b"},
		Decks: [][]*cards.Card{mountainDeck(t, 40), mountainDeck(t, 40)}})
	got := e.abilityCompiledCost(sa)
	if got != first {
		t.Fatalf("precondition: the engine path did not read the slot entry AbilityCostOf stored")
	}
	if got.Cost.XColors != 1<<2 {
		t.Fatalf("XColors = %#b after an AbilityCostOf read first, want the black mask (bit 2)", got.Cost.XColors)
	}
	if paid := got.Cost.WithX(2); paid.Colored[2] != 2 || paid.Generic != 0 {
		t.Fatalf("X = 2 paid as %+v, want two black pips and no generic", paid)
	}
}

// TestWithSpellAbilityExtrasKeepsIncomingXColor: a face without XColor$ must
// not clear a mask the incoming cost already carries (split.go nests two
// WithSpellAbilityExtras calls, the outer face over the inner's result).
func TestWithSpellAbilityExtrasKeepsIncomingXColor(t *testing.T) {
	const src = "Name:Plain Probe\nManaCost:X R\nTypes:Sorcery\n" +
		"A:SP$ DealDamage | NumDmg$ X | ValidTgts$ Any | SpellDescription$ x\nOracle:x\n"
	c, diags := cards.ParseBytes("plain_probe.txt", []byte(src))
	if len(diags) != 0 {
		t.Fatalf("parse diags: %+v", diags)
	}
	f := c.Faces[0]
	if f.SpellAbility() == nil {
		t.Fatal("precondition: the face has no SpellAbility to fold")
	}
	in := pay.FaceCostOf(f).Cost
	in.XColors = 1 << 2
	if got := pay.WithSpellAbilityExtras(f, in); got.XColors != 1<<2 {
		t.Fatalf("XColors = %#b, want the incoming black mask kept", got.XColors)
	}
}
