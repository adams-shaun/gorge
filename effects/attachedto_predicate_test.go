package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestAttachedToLiteralPredicate is the effects leaf for the two-token space
// grammar "AttachedTo <X>", where <X> is a literal type or object class the
// base grammar can answer from the object in hand. Before this change the
// whole "AttachedTo Creature" token survived the spec splitter as one word
// (a space is neither a ',' nor a '.' nor a '+') and failed closed as a
// single unknown predicate, so a filter naming "an Aura attached to a
// creature" matched NOTHING -- the same silent-unknown the pg1 verdict found.
//
// The rule: for a candidate o (an Aura or Equipment), AttachedTo <X> holds
// when o.AttachedTo names a permanent on the battlefield (o is attached) that
// satisfies the base <X>. It is the two-token counterpart of the source-side
// attachedBy (AttachedBy/EquippedBy/EnchantedBy), which reads the SOURCE's
// AttachedTo to find what the source is attached to; here we read the
// candidate object's own AttachedTo.
func TestAttachedToLiteralPredicate(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears") // a Creature
	mountain := corpusObject(t, reg, g, "Mountain")  // a Land

	// Three distinct Aura objects: a creature-Aura (Enchant creature) attached
	// to the creature, a land-Aura (Enchant land) attached to the land, and an
	// unattached Aura.
	auraOnBear := corpusObject(t, reg, g, "Unholy Strength")
	auraOnBear.AttachedTo = bear.ID
	auraOnLand := corpusObject(t, reg, g, "Wild Growth")
	auraOnLand.AttachedTo = mountain.ID
	auraFree := corpusObject(t, reg, g, "Unholy Strength")

	// AttachedTo Creature: exactly the Aura attached to the creature.
	if !MatchesObjectCtx(g, "Aura.AttachedTo Creature", auraOnBear, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Creature must match an Aura attached to a creature")
	}
	if MatchesObjectCtx(g, "Aura.AttachedTo Creature", auraOnLand, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Creature must not match an Aura attached to a land")
	}
	if MatchesObjectCtx(g, "Aura.AttachedTo Creature", auraFree, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Creature must not match an unattached Aura")
	}

	// AttachedTo Land: exactly the Aura attached to the land.
	if !MatchesObjectCtx(g, "Aura.AttachedTo Land", auraOnLand, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Land must match an Aura attached to a land")
	}
	if MatchesObjectCtx(g, "Aura.AttachedTo Land", auraOnBear, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Land must not match an Aura attached to a creature")
	}

	// AttachedTo Card: the "Card" base answers anything, so any attached
	// (non-nil AttachedTo) permanent satisfies it; an unattached Aura does
	// not, because there is no object to test.
	if !MatchesObjectCtx(g, "Aura.AttachedTo Card", auraOnBear, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Card must match an Aura attached to a permanent")
	}
	if !MatchesObjectCtx(g, "Aura.AttachedTo Card", auraOnLand, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Card must match an Aura attached to a land")
	}
	if MatchesObjectCtx(g, "Aura.AttachedTo Card", auraFree, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Card must not match an unattached Aura")
	}

	// AttachedTo Permanent: the object class answers "is on the battlefield";
	// both attachments here are battlefield permanents, so both match and the
	// unattached Aura does not.
	if !MatchesObjectCtx(g, "Aura.AttachedTo Permanent", auraOnBear, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Permanent must match an Aura attached to a permanent")
	}
	if MatchesObjectCtx(g, "Aura.AttachedTo Permanent", auraFree, SpecContext{You: 0}) {
		t.Errorf("Aura.AttachedTo Permanent must not match an unattached Aura")
	}

	// Leaf 3: the leading-'!' path (pg1) negates the recognised positive
	// predicate without any special-casing here. !AttachedTo Creature holds
	// for the Aura attached to the land and for the unattached Aura, and not
	// for the Aura attached to the creature.
	if !MatchesObjectCtx(g, "Aura.!AttachedTo Creature", auraOnLand, SpecContext{You: 0}) {
		t.Errorf("Aura.!AttachedTo Creature must match an Aura attached to a land (it is not attached to a creature)")
	}
	if !MatchesObjectCtx(g, "Aura.!AttachedTo Creature", auraFree, SpecContext{You: 0}) {
		t.Errorf("Aura.!AttachedTo Creature must match an unattached Aura (it is not attached to a creature)")
	}
	if MatchesObjectCtx(g, "Aura.!AttachedTo Creature", auraOnBear, SpecContext{You: 0}) {
		t.Errorf("Aura.!AttachedTo Creature must not match an Aura attached to a creature")
	}

	// The matcher and UnknownPredicates agree: AttachedTo <literal> and its
	// '!' negation are recognised shapes, so the census does not report them.
	for _, spec := range []string{
		"Aura.AttachedTo Creature", "Aura.AttachedTo Card", "Aura.AttachedTo Permanent",
		"Aura.AttachedTo Land", "Aura.!AttachedTo Creature",
	} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want empty (recognised AttachedTo literal)", spec, un)
		}
	}
}

// TestAttachedToTargetedStillUnknown is the leaf that keeps the scope honest:
// AttachedTo Targeted (the self-referential resolution-time referent pg1 puts
// in step 4) must keep failing closed -- it matches nothing and is still
// reported by UnknownPredicates -- so this work does not silently pretend the
// referent grammar landed. The nested "AttachedTo Permanent.YouCtrl" shape
// (a predicate on the attached object, not a single literal) is the same
// class and also stays unknown.
func TestAttachedToTargetedStillUnknown(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})
	bear := corpusObject(t, reg, g, "Grizzly Bears")
	aura := corpusObject(t, reg, g, "Unholy Strength")
	aura.AttachedTo = bear.ID

	for _, spec := range []string{"Aura.AttachedTo Targeted", "Aura.AttachedTo Permanent.YouCtrl", "Aura.AttachedTo TriggeredCardLKICopy"} {
		if MatchesObjectCtx(g, spec, aura, SpecContext{You: 0}) {
			t.Errorf("%s must match nothing (the referent needs resolution-time context)", spec)
		}
	}
	for spec, want := range map[string]string{
		"Aura.AttachedTo Targeted":             "AttachedTo Targeted",
		"Aura.AttachedTo Permanent.YouCtrl":    "AttachedTo Permanent.YouCtrl",
		"Aura.AttachedTo TriggeredCardLKICopy": "AttachedTo TriggeredCardLKICopy",
	} {
		un := UnknownPredicates(spec)
		if len(un) != 1 || un[0] != want {
			t.Errorf("UnknownPredicates(%q) = %v, want [%s]", spec, un, want)
		}
	}
}
