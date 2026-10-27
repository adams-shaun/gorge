package effects

// The card-type share family (sharesCardTypeWith, sharesCardTypeWithOther,
// sharesAllCardTypesWithOther) reads the layer-4 (CR 613.1d) card-type set on
// BOTH sides: the candidate through derivedCardTypes, the referent through
// hasTypeCtx. A derived entry is authoritative and is not unioned with the
// printed face, so an animated land (Mutavault) or a Karn-animated artifact
// gains Creature, and a RemoveCardTypes$ loses a word. Only an object with no
// derived entry -- off the battlefield -- falls back to its printed face.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// sharesCardTypeBoard is a battlefield holding the shapes the family's
// layer-4 read has to distinguish: a Bear (Creature), a Mutavault (printed
// Land), a Forest (Land), a Lightning Bolt (Instant), an artifact creature
// and a bare artifact.
type sharesCardTypeBoard struct {
	h                            *fakeHost
	bear, vault, forest, bolt    state.ObjID
	artifactCreature, artifactID state.ObjID
}

func newSharesCardTypeBoard(t *testing.T) sharesCardTypeBoard {
	t.Helper()
	h := newHost(t, 2)
	g := h.g
	bear := g.AddObject(mkCard(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	vault := g.AddObject(mkCard(t, "Name:Mutavault\nTypes:Land\nOracle:x\n"), 0)
	forest := g.AddObject(mkCard(t, "Name:Forest\nTypes:Land Forest\nOracle:x\n"), 0)
	bolt := g.AddObject(mkCard(t, "Name:Lightning Bolt\nTypes:Instant\nOracle:x\n"), 0)
	artifactCreature := g.AddObject(mkCard(t, "Name:Test Golem\nTypes:Artifact Creature Golem\nPT:2/2\nOracle:x\n"), 0)
	artifact := g.AddObject(mkCard(t, "Name:Test Relic\nTypes:Artifact\nOracle:x\n"), 0)
	ids := []state.ObjID{bear.ID, vault.ID, forest.ID, bolt.ID, artifactCreature.ID, artifact.ID}
	for _, id := range ids {
		o := g.Obj(id)
		if o == nil {
			t.Fatalf("fixture object %d is missing", id)
		}
		o.Zone = state.ZBattlefield
	}
	g.SetZone(state.ZBattlefield, 0, ids)
	return sharesCardTypeBoard{
		h: h, bear: bear.ID, vault: vault.ID, forest: forest.ID, bolt: bolt.ID,
		artifactCreature: artifactCreature.ID, artifactID: artifact.ID,
	}
}

// derivedVault is the layer-4 entry a Mutavault animation binds (its printed
// Land plus the granted Creature), keyed to the fixture's Mutavault.
func derivedVault(vault state.ObjID) LayerTables {
	return LayerTables{DerivedTypes: []ObjectTypes{{ID: vault, Types: []string{"Land", "Creature"}}}}
}

// TestSharesCardTypeDerivedReferent pins the REFERENT side: an animated land
// shares Creature with a Bear, while a bare Forest still shares nothing.
func TestSharesCardTypeDerivedReferent(t *testing.T) {
	b := newSharesCardTypeBoard(t)
	g := b.h.g
	const spec = "Card.sharesCardTypeWith Remembered"
	const specOther = "Card.sharesCardTypeWithOther Remembered"

	// Precondition: the objects sit in the zone the rule reads, and with NO
	// derived entry the printed Land shares no card type with the Bear.
	if o := g.Obj(b.vault); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: the Mutavault must be on the battlefield, got %+v", o)
	}
	plain := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.vault}}}
	if MatchesObjectCtx(g, spec, g.Obj(b.bear), plain) {
		t.Fatal("precondition: a printed Land referent must not share Creature with the Bear")
	}
	if MatchesObjectCtx(g, specOther, g.Obj(b.bear), plain) {
		t.Fatal("precondition: a printed Land referent must not share Creature with the Bear (Other)")
	}

	sc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.vault}}, Layers: derivedVault(b.vault)}
	if !MatchesObjectCtx(g, spec, g.Obj(b.bear), sc) {
		t.Error("the Bear must share Creature with the animated land referent")
	}
	if !MatchesObjectCtx(g, specOther, g.Obj(b.bear), sc) {
		t.Error("the Bear must share Creature with the animated land referent (Other)")
	}

	// Negative: a Forest referent stays a bare Land even while the derived
	// table binds the Mutavault.
	forestSc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.forest}}, Layers: sc.Layers}
	if MatchesObjectCtx(g, spec, g.Obj(b.bear), forestSc) {
		t.Error("the Bear must not share a card type with a Forest referent")
	}
	if MatchesObjectCtx(g, specOther, g.Obj(b.bear), forestSc) {
		t.Error("the Bear must not share a card type with a Forest referent (Other)")
	}
}

// TestSharesCardTypeDerivedCandidate pins the CANDIDATE side: an animated
// land candidate shares Creature with a Bear, while an Instant referent
// shares nothing.
func TestSharesCardTypeDerivedCandidate(t *testing.T) {
	b := newSharesCardTypeBoard(t)
	g := b.h.g
	const spec = "Card.sharesCardTypeWith Remembered"
	const specOther = "Card.sharesCardTypeWithOther Remembered"

	// Precondition: no derived entry, the printed Land candidate shares no
	// card type with the Bear referent.
	plain := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.bear}}}
	if MatchesObjectCtx(g, spec, g.Obj(b.vault), plain) {
		t.Fatal("precondition: a printed Land candidate must not share Creature with the Bear")
	}
	if MatchesObjectCtx(g, specOther, g.Obj(b.vault), plain) {
		t.Fatal("precondition: a printed Land candidate must not share Creature with the Bear (Other)")
	}

	sc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.bear}}, Layers: derivedVault(b.vault)}
	if !MatchesObjectCtx(g, spec, g.Obj(b.vault), sc) {
		t.Error("the animated land must share Creature with the Bear referent")
	}
	if !MatchesObjectCtx(g, specOther, g.Obj(b.vault), sc) {
		t.Error("the animated land must share Creature with the Bear referent (Other)")
	}

	// Negative: an Instant referent shares no card type with the land.
	boltSc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.bolt}}, Layers: sc.Layers}
	if MatchesObjectCtx(g, spec, g.Obj(b.vault), boltSc) {
		t.Error("the animated land must not share a card type with an Instant referent")
	}
	if MatchesObjectCtx(g, specOther, g.Obj(b.vault), boltSc) {
		t.Error("the animated land must not share a card type with an Instant referent (Other)")
	}
}

// TestSharesAllCardTypesDerivedReferent pins the all-types read's referent
// side: a Creature-only Bear shares all its card types with an animated land
// that now has Creature, but not with a bare Forest.
func TestSharesAllCardTypesDerivedReferent(t *testing.T) {
	b := newSharesCardTypeBoard(t)
	g := b.h.g
	const spec = "Card.sharesAllCardTypesWithOther Remembered"

	// Precondition: with no derived entry the printed Land referent is
	// missing the Bear's Creature.
	plain := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.vault}}}
	if MatchesObjectCtx(g, spec, g.Obj(b.bear), plain) {
		t.Fatal("precondition: a printed Land must not share all its card types with the Bear")
	}

	sc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.vault}}, Layers: derivedVault(b.vault)}
	if !MatchesObjectCtx(g, spec, g.Obj(b.bear), sc) {
		t.Error("the Bear must share all its card types with the animated land referent")
	}

	forestSc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.forest}}, Layers: sc.Layers}
	if MatchesObjectCtx(g, spec, g.Obj(b.bear), forestSc) {
		t.Error("the Bear must not share all its card types with a Forest referent")
	}
}

// TestSharesAllCardTypesDerivedCandidate pins the false-positive direction:
// before the animation an animated-land candidate prints only Land and so
// wrongly shares all its card types with a bare Forest; the derived
// {Land, Creature} list makes the comparison false.
func TestSharesAllCardTypesDerivedCandidate(t *testing.T) {
	b := newSharesCardTypeBoard(t)
	g := b.h.g
	const spec = "Card.sharesAllCardTypesWithOther Remembered"

	// Precondition: the compared values differ -- the printed Land candidate
	// IS a subset of the Forest's Land, so the relation holds before the
	// animation is bound.
	plain := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.forest}}}
	if !MatchesObjectCtx(g, spec, g.Obj(b.vault), plain) {
		t.Fatal("precondition: a printed Land candidate must share all its card types with a Forest")
	}

	sc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.forest}}, Layers: derivedVault(b.vault)}
	if MatchesObjectCtx(g, spec, g.Obj(b.vault), sc) {
		t.Error("the animated land must not share all its card types (Land, Creature) with a bare Forest")
	}
}

// TestSharesCardTypeDerivedRemoval pins that a derived entry is authoritative
// rather than a union with the printed face: a printed Artifact Creature with
// RemoveCardTypes$ for Creature no longer shares Creature with a Bear, but
// still shares Artifact with an artifact. The candidate side is the same
// shape.
func TestSharesCardTypeDerivedRemoval(t *testing.T) {
	b := newSharesCardTypeBoard(t)
	g := b.h.g
	const spec = "Card.sharesCardTypeWith Remembered"

	// Precondition: the printed Artifact Creature shares Creature with the
	// Bear, so the removal assertions depend on the derived entry.
	plain := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.artifactCreature}}}
	if !MatchesObjectCtx(g, spec, g.Obj(b.bear), plain) {
		t.Fatal("precondition: the printed Artifact Creature must share Creature with the Bear")
	}

	removed := LayerTables{DerivedTypes: []ObjectTypes{{ID: b.artifactCreature, Types: []string{"Artifact"}}}}

	// Referent side.
	refSc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.artifactCreature}}, Layers: removed}
	if MatchesObjectCtx(g, spec, g.Obj(b.bear), refSc) {
		t.Error("a derived Artifact-only referent must not share Creature with the Bear")
	}
	if !MatchesObjectCtx(g, spec, g.Obj(b.artifactID), refSc) {
		t.Error("a derived Artifact-only referent must still share Artifact with an artifact")
	}

	// Candidate side: the same removed word, read from the candidate.
	candSc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.bear}}, Layers: removed}
	if MatchesObjectCtx(g, spec, g.Obj(b.artifactCreature), candSc) {
		t.Error("a derived Artifact-only candidate must not share Creature with the Bear")
	}
	candArtSc := SpecContext{You: 0, Remembered: []state.Target{{Obj: b.artifactID}}, Layers: removed}
	if !MatchesObjectCtx(g, spec, g.Obj(b.artifactCreature), candArtSc) {
		t.Error("a derived Artifact-only candidate must still share Artifact with an artifact")
	}
}
