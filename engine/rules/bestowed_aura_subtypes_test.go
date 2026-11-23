package rules

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestBestowedAuraHasNoCreatureSubtypes pins CR 205.1a / 702.103b on a real
// carrier: while Nyxborn Hydra is bestowed on a creature it is an Aura and not
// a creature, and the creature subtypes correlated with the lost Creature card
// type (its Hydra subtype) go with it. The derived type line is Enchantment
// Aura.
func TestBestowedAuraHasNoCreatureSubtypes(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	hydra := mustCorpusCard(t, reg, "Nyxborn Hydra")
	e, cfg := tokenReplGame(t, 1454, hydra)
	hydraID := moveSeededCard(t, e, 0, hydra, state.ZHand)
	bear := putToken(t, e, 0, bestowBearerSrc, state.ZBattlefield)

	// Precondition: the printed card is a Creature with the Hydra subtype, so
	// the assertion below is about the bestowed switch and not an empty list.
	printed := e.G.Obj(hydraID).Face().Types
	if !slices.Contains(printed, "Creature") || !slices.Contains(printed, "Hydra") {
		t.Fatalf("printed Hydra types %v, want Creature and Hydra", printed)
	}

	addMana(t, e, 0, "GGG") // {X=1}{G}{G}
	submitChoices(t, e, bestowedCastOption(t, e, hydraID).Index)
	d := e.Pending()
	if d == nil || d.Kind != decision.KChoose {
		t.Fatalf("bestowed hydra did not announce X: %+v", d)
	}
	x1 := -1
	for _, o := range d.Options {
		if o.Label == "X = 1" {
			x1 = o.Index
		}
	}
	if x1 < 0 {
		t.Fatalf("no X = 1 option on the announcement: %+v", d.Options)
	}
	submitChoices(t, e, x1)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("bestowed hydra target ask: %+v", d)
	}
	tgt := indexOfObjOption(d, bear)
	if tgt < 0 {
		t.Fatalf("bear not offered as a bestow target: %+v", d.Options)
	}
	submitChoices(t, e, tgt)
	passUntilStackEmpty(t, e, 20)

	o := e.G.Obj(hydraID)
	if o.Zone != state.ZBattlefield || o.AttachedTo != bear {
		t.Fatalf("hydra zone %s attached %d, want battlefield/bear %d", o.Zone, o.AttachedTo, bear)
	}
	types := e.Derived(hydraID).Types
	if e.IsCreature(hydraID) || slices.Contains(types, "Creature") {
		t.Fatalf("bestowed-attached Hydra is still a creature: derived types %v", types)
	}
	if !slices.Contains(types, "Enchantment") || !slices.Contains(types, "Aura") {
		t.Fatalf("bestowed Hydra types %v, want Enchantment and Aura", types)
	}
	if slices.Contains(types, "Hydra") {
		t.Fatalf("bestowed Aura keeps the Hydra creature subtype (CR 205.1a): derived types %v", types)
	}
	replayCheck(t, e, cfg)
}
