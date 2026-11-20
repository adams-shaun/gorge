package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// gatedCloneFixture is an inline card whose ETBReplacement:Copy DB$ Clone body
// the selector declines: PumpDuration$ names a window (until your next turn)
// the Clone resolution does not implement, so offering the election would
// leave the haste outliving it.
const gatedCloneFixture = `Name:Gated Clone Fixture
ManaCost:2 U
Types:Creature Shapeshifter
PT:0/0
K:ETBReplacement:Copy:FixtureCopy:Optional
SVar:FixtureCopy:DB$ Clone | PumpDuration$ UntilYourNextTurn | PumpKeywords$ Haste | Choices$ Creature.Other
Oracle:Fixture.
`

// TestETBCopyShapeGateCountsLoudNoteCarriersUnsupported pins the coverage
// report's shape gate: a card whose ETBReplacement:Copy DB$ Clone body the
// engine declines (the fixture's PumpDuration$ window; its cast keeps the loud
// unimplemented-API fallback) is primitive-complete but takes the loud
// unimplemented-API Note when cast, so the gate must count it unsupported;
// Phantasmal Image (AddTypes$/AddTriggers$/AddSVars$ riders, now offered),
// Vesuvan Shapeshifter (UntilFacedown, now offered), Vizier of Many Faces (the
// Embalm$ provenance gate, now offered) and The Fourteenth Doctor
// (PumpKeywords$ Haste until end of turn, now offered) must stay playable.
func TestETBCopyShapeGateCountsLoudNoteCarriersUnsupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	gated, diags := cards.ParseBytes("gated.txt", []byte(gatedCloneFixture))
	if len(diags) != 0 {
		t.Fatalf("fixture diagnostics: %v", diags)
	}
	gated.Link()
	for _, f := range gated.Faces {
		f.ApplyIntrinsics()
	}
	image, ok2 := r.Lookup("Phantasmal Image")
	vizier, ok3 := r.Lookup("Vizier of Many Faces")
	doctor, ok4 := r.Lookup("The Fourteenth Doctor")
	if !ok2 || !ok3 || !ok4 {
		t.Fatal("corpus missing Phantasmal Image / Vizier of Many Faces / The Fourteenth Doctor")
	}
	// Precondition: the fixture is primitive-complete, so only the gate can differ.
	if m := r.Unsupported(gated, sup); len(m) != 0 {
		t.Fatalf("the fixture is unexpectedly unsupported by primitive: %v", m)
	}
	if len(etbCopyShapeGate(gated)) == 0 {
		t.Fatal("the fixture's declined ETB Clone body is not gated")
	}
	for name, c := range map[string]*cards.Card{"Phantasmal Image": image, "Vizier of Many Faces": vizier, "The Fourteenth Doctor": doctor} {
		if g := etbCopyShapeGate(c); len(g) != 0 {
			t.Errorf("%s gated: %v", name, g)
		}
	}
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	if len(r.Unsupported(gated, sup)) == 0 {
		t.Error("the fixture counted playable with the gate set")
	}
	for name, c := range map[string]*cards.Card{"Phantasmal Image": image, "Vizier of Many Faces": vizier, "The Fourteenth Doctor": doctor} {
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s counted unsupported: %v", name, m)
		}
	}
}
