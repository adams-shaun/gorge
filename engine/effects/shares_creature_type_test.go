package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The sharesCreatureTypeWith leaf (Heirloom Blade's
// "Creature.sharesCreatureTypeWith TriggeredCardLKICopy"): the token joins
// the sharesCardTypeWith family's referent switch, the intersection is over
// CREATURE subtypes (not card types), and UnknownPredicates goes through the
// SAME classifier as the matcher — done-criterion 1, so the census cannot
// disagree with the matcher.

// sharesCreatureTypeFixture builds a game with the dead trigger card (a
// Bear), a matching Bear Cub and a non-matching Hill Giant in seat 0's
// library. Returns (game, dead, cub, giant).
func sharesCreatureTypeFixture(t *testing.T) (*state.Game, state.ObjID, state.ObjID, state.ObjID) {
	t.Helper()
	g := state.NewGame(names(2))
	dead := g.AddObject(mkCard(t, "Name:Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	cub := g.AddObject(mkCard(t, "Name:Bear Cub\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	giant := g.AddObject(mkCard(t, "Name:Hill Giant\nTypes:Creature Giant\nPT:3/3\nOracle:x\n"), 0)
	g.SetZone(state.ZGraveyard, 0, []state.ObjID{dead.ID})
	g.SetZone(state.ZLibrary, 0, []state.ObjID{cub.ID, giant.ID})
	return g, dead.ID, cub.ID, giant.ID
}

// TestSharesCreatureTypeWithClassified keeps the seven recognised referents
// silent to UnknownPredicates and everything else fail-closed-loud.
func TestSharesCreatureTypeWithClassified(t *testing.T) {
	for _, ref := range []string{"RememberedCard", "Remembered", "RememberedLKI",
		"TriggeredCard", "TriggeredCardLKICopy", "Targeted", "Self", "Commander"} {
		spec := "Creature.sharesCreatureTypeWith " + ref
		if got := UnknownPredicates(spec); len(got) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want []", spec, got)
		}
	}
	// The exotic referents the brief scopes OUT stay unknown (fail closed).
	for _, spec := range []string{
		"Creature.sharesCreatureTypeWith Sacrificed",
		"Creature.sharesCreatureTypeWith TriggeredAttacker",
		"Creature.sharesCreatureTypeWithAll Tapped",
	} {
		if got := UnknownPredicates(spec); len(got) == 0 {
			t.Errorf("UnknownPredicates(%q) = [], want the token reported", spec)
		}
	}
	// The card-type reading keeps its own classification.
	if got := UnknownPredicates("Creature.sharesCardTypeWith RememberedCard"); len(got) != 0 {
		t.Errorf("UnknownPredicates card-type = %v, want []", got)
	}
}

// TestSharesCreatureTypeWithMatchesTheTriggerCard is the matcher half: a Bear
// candidate shares the dead Bear's subtype; a Giant does not; an unbound
// referent matches nothing (fail closed, never widened).
func TestSharesCreatureTypeWithMatchesTheTriggerCard(t *testing.T) {
	g, dead, cub, giant := sharesCreatureTypeFixture(t)
	sc := SpecContext{You: 0}
	sc.TriggerCard = dead
	if !MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith TriggeredCardLKICopy", cub, sc) {
		t.Errorf("Bear Cub must share the dead Bear's creature type")
	}
	if MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith TriggeredCardLKICopy", giant, sc) {
		t.Errorf("Hill Giant must not share the dead Bear's creature type")
	}
	scUnbound := SpecContext{You: 0}
	if MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith TriggeredCardLKICopy", cub, scUnbound) {
		t.Errorf("an unbound referent must match nothing (fail closed)")
	}
	// The card-type reading is unchanged by the creature twin: both Bear Cub
	// and Hill Giant are Creatures, so the CARD-type spec matches both.
	if !MatchesSpecCtx(g, "Creature.sharesCardTypeWith TriggeredCardLKICopy", giant, sc) {
		t.Errorf("Hill Giant shares the CARD type Creature with the dead Bear")
	}
}

// TestSharesCreatureTypeWithCommander pins the Commander referent (Path of
// Ancestry's "a creature spell that shares a creature type with your
// commander", the corpus's only carrier): it resolves to the SOURCE's
// controller's commanders, and an unbound source (or a seat with no
// commanders) matches nothing -- fail closed, never widened.
func TestSharesCreatureTypeWithCommander(t *testing.T) {
	g := state.NewGame(names(2))
	src := g.AddObject(mkCard(t, "Name:Path of Ancestry\nTypes:Land\nOracle:x\n"), 0)
	cmdr := g.AddObject(mkCard(t, "Name:Goblin Commander\nTypes:Legendary Creature Goblin Shaman\nPT:2/2\nOracle:x\n"), 0)
	g.Players[0].Commanders = []state.ObjID{cmdr.ID}
	goblin := g.AddObject(mkCard(t, "Name:Goblin Guide\nTypes:Creature Goblin Scout\nPT:2/1\nOracle:x\n"), 0)
	bear := g.AddObject(mkCard(t, "Name:Bear Cub\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)

	// The reference is recognised as a known predicate (matcher and census
	// share the classifier).
	if got := UnknownPredicates("Creature.sharesCreatureTypeWith Commander"); len(got) != 0 {
		t.Errorf("UnknownPredicates(Commander) = %v, want []", got)
	}

	sc := SpecContext{You: 0, Source: src.ID}
	if !MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith Commander", goblin.ID, sc) {
		t.Errorf("Goblin Guide must share the Goblin commander's creature type")
	}
	if MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith Commander", bear.ID, sc) {
		t.Errorf("Bear Cub must not share the Goblin commander's creature type")
	}

	// An unbound source (no controller) binds nothing: fail closed.
	if MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith Commander", goblin.ID, SpecContext{You: 0}) {
		t.Errorf("an unbound Commander source must match nothing (fail closed)")
	}

	// A source with no commanders (seat 1) also binds nothing.
	src1 := g.AddObject(mkCard(t, "Name:Plain Land\nTypes:Land\nOracle:x\n"), 1)
	if MatchesSpecCtx(g, "Creature.sharesCreatureTypeWith Commander", goblin.ID, SpecContext{You: 1, Source: src1.ID}) {
		t.Errorf("a seat with no commanders must match nothing (fail closed)")
	}
}

// TestSharesCreatureTypeReferentAllTypes pins the REFERENT side of the
// layer-4 type read: a referent with every creature type (Mistform Ultimus,
// or Mutavault animated with all creature types) shares a creature type with
// any creature, and a referent whose derived list grants a word its printed
// face does not print shares that word too. Before the derived read the
// predicate looped over the referent's printed words only, so both cases
// matched nothing from this direction.
func TestSharesCreatureTypeReferentAllTypes(t *testing.T) {
	g := state.NewGame(names(2))
	ultimus := g.AddObject(mkCard(t, "Name:Mistform Ultimus\nTypes:Legendary Creature Illusion\nPT:3/3\nOracle:x\n"), 0)
	mutavault := g.AddObject(mkCard(t, "Name:Mutavault\nTypes:Land\nOracle:x\n"), 0)
	bear := g.AddObject(mkCard(t, "Name:Grizzly Bears\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"), 0)
	giant := g.AddObject(mkCard(t, "Name:Hill Giant\nTypes:Creature Giant\nPT:3/3\nOracle:x\n"), 0)

	const spec = "Creature.sharesCreatureTypeWith Self"
	if got := UnknownPredicates(spec); len(got) != 0 {
		t.Fatalf("UnknownPredicates(%q) = %v, want []", spec, got)
	}
	// Precondition: neither referent's PRINTED face carries Bear, so the
	// assertions below depend on the derived table actually being read.
	for _, ref := range []state.ObjID{ultimus.ID, mutavault.ID} {
		sc := SpecContext{You: 0, Source: ref}
		if MatchesSpecCtx(g, spec, bear.ID, sc) {
			t.Fatalf("precondition: referent %d's printed face must not share Bear", ref)
		}
	}

	// All creature types on the referent side: shares with a Bear and a
	// Giant alike (CR 702.73a).
	allTypes := SpecContext{You: 0, Source: ultimus.ID, Layers: LayerTables{DerivedTypes: []ObjectTypes{
		{ID: ultimus.ID, Types: []string{"Legendary", "Creature", "Illusion"}, AllCreatureTypes: true},
	}}}
	for _, cand := range []state.ObjID{bear.ID, giant.ID} {
		if !MatchesSpecCtx(g, spec, cand, allTypes) {
			t.Errorf("an all-creature-types referent must share with candidate %d", cand)
		}
	}

	// A derived creature word not on the printed face (an animated land's
	// grant) is one of the referent's creature types too.
	granted := SpecContext{You: 0, Source: mutavault.ID, Layers: LayerTables{DerivedTypes: []ObjectTypes{
		{ID: mutavault.ID, Types: []string{"Land", "Creature", "Bear"}},
	}}}
	if !MatchesSpecCtx(g, spec, bear.ID, granted) {
		t.Errorf("a referent with a derived Bear word must share Bear")
	}
	if MatchesSpecCtx(g, spec, giant.ID, granted) {
		t.Errorf("a referent with derived Land/Creature/Bear must not share Giant")
	}

	// No common creature word either side (both fall back to printed
	// faces): the relation stays false.
	if MatchesSpecCtx(g, spec, bear.ID, SpecContext{You: 0, Source: giant.ID}) {
		t.Errorf("a Giant referent and a Bear candidate share no creature type")
	}
}
