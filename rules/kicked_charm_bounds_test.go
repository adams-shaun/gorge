package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
)

func TestInscriptionOfAbundanceKickedCharmBounds(t *testing.T) {
	reg := searchTestRegistry(t)
	card := searchCorpusCard(t, reg, "Inscription of Abundance")
	if len(card.Faces) == 0 {
		t.Fatal("Inscription of Abundance has no face")
	}
	face := card.Faces[0]
	sa := face.SpellAbility()
	if sa == nil {
		t.Fatal("Inscription of Abundance has no compiled spell ability")
	}
	e, _ := searchEngine(t, reg, "Inscription of Abundance")
	ctx := &effects.Ctx{PendingKicked: true}
	effects.SetSVars(ctx, face.SVars)
	min, max, _ := effects.CharmModeBounds(e, ctx, sa, 3)
	if min != 1 || max != 3 {
		t.Fatalf("kicked mode bounds = %d..%d, want 1..3", min, max)
	}
	ctx.PendingKicked = false
	min, max, _ = effects.CharmModeBounds(e, ctx, sa, 3)
	if min != 1 || max != 1 {
		t.Fatalf("unkicked mode bounds = %d..%d, want 1..1", min, max)
	}
}
