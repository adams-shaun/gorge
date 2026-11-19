package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestETBCopyShapeGateCountsLoudNoteCarriersUnsupported pins the coverage
// report's shape gate: a card whose ETBReplacement:Copy DB$ Clone body the
// engine declines (The Fourteenth Doctor's ChoiceZone$/PumpKeywords$ riders,
// whose cast keeps the loud fallback per rules'
// TestFourteenthDoctorETBKeepsLoudFallback) is primitive-complete but takes
// the loud unimplemented-API Note when cast, so the gate must count it
// unsupported; Phantasmal Image (AddTypes$/AddTriggers$/AddSVars$ riders, now
// offered), Vesuvan Shapeshifter (UntilFacedown, now offered) and Vizier of
// Many Faces (the Embalm$ provenance gate, now offered) must stay playable.
func TestETBCopyShapeGateCountsLoudNoteCarriersUnsupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	doctor, ok1 := r.Lookup("The Fourteenth Doctor")
	image, ok2 := r.Lookup("Phantasmal Image")
	vizier, ok3 := r.Lookup("Vizier of Many Faces")
	if !ok1 || !ok2 || !ok3 {
		t.Fatal("corpus missing The Fourteenth Doctor / Phantasmal Image / Vizier of Many Faces")
	}
	// Precondition: all are primitive-complete, so only the gate can differ.
	if m := r.Unsupported(doctor, sup); len(m) != 0 {
		t.Fatalf("The Fourteenth Doctor unexpectedly unsupported by primitive: %v", m)
	}
	if len(etbCopyShapeGate(doctor)) == 0 {
		t.Fatal("The Fourteenth Doctor's declined ETB Clone body is not gated")
	}
	if g := etbCopyShapeGate(image); len(g) != 0 {
		t.Fatalf("Phantasmal Image gated: %v", g)
	}
	if g := etbCopyShapeGate(vizier); len(g) != 0 {
		t.Fatalf("Vizier of Many Faces still gated: %v", g)
	}
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	if len(r.Unsupported(doctor, sup)) == 0 {
		t.Error("The Fourteenth Doctor counted playable with the gate set")
	}
	if m := r.Unsupported(image, sup); len(m) != 0 {
		t.Errorf("Phantasmal Image counted unsupported: %v", m)
	}
	if m := r.Unsupported(vizier, sup); len(m) != 0 {
		t.Errorf("Vizier of Many Faces counted unsupported: %v", m)
	}
}
