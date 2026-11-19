package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredSpellManaValueKeepsXAfterCounter: Dovescape counters the spell
// that fired it and then creates X Birds, X being that spell's mana value
// (TriggeredSpellAbility$CardManaCostLKI). The Counter moves the spell off the
// stack, where events.Apply zeroes its announced X, so the live read gave the
// printed value (2 for X R R) where CR 608.2h's last-known information is 4.
func TestTriggeredSpellManaValueKeepsXAfterCounter(t *testing.T) {
	registerSpellMVProbe(t)
	h, c := fixtureHost(t)
	spell := mkCard(t, "Name:X Spell\nManaCost:X R R\nTypes:Instant\nOracle:x\n")
	obj := h.g.AddObject(spell, 1)
	sp := h.g.Obj(obj.ID)
	sp.Zone = state.ZStack
	sp.X = 2
	h.g.SetZone(state.ZStack, 1, []state.ObjID{obj.ID})
	c.Targets = []state.Target{{Obj: obj.ID}}
	c.Remembered = []state.Target{{Obj: obj.ID}}
	if got := state.ManaValue(h.g, h.g.Obj(obj.ID)); got != 4 {
		t.Fatalf("precondition: stack mana value = %d, want 4 (X=2)", got)
	}

	probeSpellMV, probeSpellMVOK = -1, false
	Resolve(h, c, spellMVProbe(t, "SP$ Counter"))
	if o := h.g.Obj(obj.ID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("postcondition: spell = %+v, want countered into the graveyard", h.g.Obj(obj.ID))
	}
	if live := state.ManaValue(h.g, h.g.Obj(obj.ID)); live != 2 {
		t.Fatalf("postcondition: live mana value after the move = %d, want 2 (X lost), else the snapshot is not what answers", live)
	}
	probeSpellMV, probeSpellMVOK = EvalCountOK(h, c, "TriggeredSpellAbility$CardManaCostLKI")
	if !probeSpellMVOK || probeSpellMV != 4 {
		t.Errorf("TriggeredSpellAbility$CardManaCostLKI after Counter = (%d,%v), want (4,true)", probeSpellMV, probeSpellMVOK)
	}
}

// TestSpellTargetedManaValueKeepsXAfterCounter drives refManaValue's snapshot
// arm itself: SpellTargeted$CardManaCostLKI (Reject Imperfection, Sound the
// Trumpets) reaches the property read through refTargetsSpellTargeted, with no
// capture keyed on the spelling (the TriggeredSpellAbility spelling is
// answered by a Remembered capture and never gets there). The chain counters
// an X=2 "X R R" spell and reads its mana value after: the live read is 2
// (announced X zeroed by the move), CR 608.2h's last-known value is 4.
func TestSpellTargetedManaValueKeepsXAfterCounter(t *testing.T) {
	registerSpellMVProbe(t)
	h, c := fixtureHost(t)
	spell := mkCard(t, "Name:X Spell\nManaCost:X R R\nTypes:Instant\nOracle:x\n")
	obj := h.g.AddObject(spell, 1)
	sp := h.g.Obj(obj.ID)
	sp.Zone = state.ZStack
	sp.X = 2
	h.g.SetZone(state.ZStack, 1, []state.ObjID{obj.ID})
	c.Targets = []state.Target{{Obj: obj.ID}}
	if got := state.ManaValue(h.g, h.g.Obj(obj.ID)); got != 4 {
		t.Fatalf("precondition: stack mana value = %d, want 4 (X=2)", got)
	}
	if len(c.Remembered) != 0 {
		t.Fatalf("precondition: Remembered = %v, want none (only the target snapshot may answer)", c.Remembered)
	}

	probeSpellMV, probeSpellMVOK = -1, false
	Resolve(h, c, spellMVProbe(t, "SP$ Counter"))
	if o := h.g.Obj(obj.ID); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("postcondition: spell = %+v, want countered into the graveyard", h.g.Obj(obj.ID))
	}
	if live := state.ManaValue(h.g, h.g.Obj(obj.ID)); live != 2 {
		t.Fatalf("postcondition: live mana value after the move = %d, want 2 (X lost), else the snapshot is not what answers", live)
	}
	if !probeSpellMVOK || probeSpellMV != 4 {
		t.Errorf("SpellTargeted$CardManaCostLKI after Counter = (%d,%v), want (4,true)", probeSpellMV, probeSpellMVOK)
	}
	// The Targeted spelling (Sound the Trumpets' sibling reads, Press the
	// Enemy) goes through the same property arm.
	if got, ok := EvalCountOK(h, c, "Targeted$CardManaCostLKI"); !ok || got != 4 {
		t.Errorf("Targeted$CardManaCostLKI after Counter = (%d,%v), want (4,true)", got, ok)
	}
}
