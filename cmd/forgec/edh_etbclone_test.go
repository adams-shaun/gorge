package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edhETBCloneCards is the EDH census class "ETBReplacement:Copy except riders
// the clone shape gate declines": every card's as-enters DB$ Clone body must
// be offered (effects.CloneETBOffered) so the gate no longer counts it
// unplayable.
//
// Infinite Reflection's body is the foreign-entry shape (an ETBReplacement on
// OTHER entering creatures), offered by effects.CloneForeignEntryOffered.
//
// Vesuvan Shapeshifter's Duration$ UntilFacedown is admitted: the copy rides
// rules' event-driven expiry at its granted upkeep trigger's TurnFaceDown,
// and its printed TurnFaceUp replacement re-offers the copy
// (clone_vesuvan_facedown_test.go drives the whole loop).
// Vizier of Many Faces' Embalm$ provenance rider ("if it was embalmed") is
// offered since the Embalm$ gate: the gated RemoveCost$/SetColor$/AddTypes$
// exceptions apply only to the card's own embalm token
// (engine/rules/edh_etbclone_vizier_test.go).
var edhETBCloneCards = []string{
	"Spark Double",
	"Cursed Mirror",
	"Imposter Mech",
	"Wall of Stolen Identity",
	"Estrid's Invocation",
	"Moritte of the Frost",
	"Machine God's Effigy",
	"Vesuvan Shapeshifter",
	"Infinite Reflection",
	"Vizier of Many Faces",
}

func TestEDHETBCloneRidersSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range edhETBCloneCards {
		c, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %s", name)
			continue
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
