package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/state"
)

// TestNonPredicateColourChosenAndCopiedSpell pins the three negation shapes
// the generic non<X> classifier must answer beyond type words and the five
// colours:
//
//   - Card.nonColorless (Blessing of the Nephilim's `Card.nonColorless`) is a
//     colour-IDENTITY test: it matches an object with at least one colour, not
//     an object with no colour. CR 105.2 -- colourless is not a colour.
//   - Land.nonChosenCard (Keldon Firebombers' `Land.nonChosenCard`) is the
//     negation of the resolution's chosen-card membership; it matches a land
//     that is NOT one of the chosen cards and does not match the chosen one.
//   - Card.nonCopiedSpell (The Heron Moon's `Card.OppOwn+!token+nonCopiedSpell`)
//     excludes a CR 707 copy of a spell: it matches a non-copy and not a copy.
//
// Every leaf asserts its own precondition (the object is in the zone the rule
// reads; the compared colour sets / chosen lists actually differ) so a vacuous
// setup fails loudly, and the three tokens are also asserted recognised by
// UnknownPredicates so the matcher and the census cannot disagree.
//
// corpusObject writes into g.Objs, which reallocates as objects are added, so
// every object is addressed by ID and re-fetched through g.Obj at each use,
// never through a pointer held across another AddObject.
func TestNonPredicateColourChosenAndCopiedSpell(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "them"})

	// Leaf 1: nonColorless is "has at least one colour". Terminate is {B}{R};
	// Expedition Map has no coloured mana symbol.
	colouredID := corpusObject(t, reg, g, "Terminate").ID        // {B}{R}
	colourlessID := corpusObject(t, reg, g, "Expedition Map").ID // colourless
	if got := ColorsOf(g.Obj(colouredID)); got == "" {
		t.Fatalf("precondition: Terminate must be coloured, ColorsOf = %q", got)
	}
	if got := ColorsOf(g.Obj(colourlessID)); got != "" {
		t.Fatalf("precondition: Expedition Map must be colourless, ColorsOf = %q", got)
	}
	if !MatchesObjectCtx(g, "Card.nonColorless", g.Obj(colouredID), SpecContext{You: 0}) {
		t.Errorf("Card.nonColorless must match a coloured card (Terminate)")
	}
	if MatchesObjectCtx(g, "Card.nonColorless", g.Obj(colourlessID), SpecContext{You: 0}) {
		t.Errorf("Card.nonColorless must not match a colourless card (Expedition Map)")
	}

	// Leaf 2: nonChosenCard is "not one of the resolution's chosen cards".
	// Keldon Firebombers sacrifices `Land.nonChosenCard`. ChosenValid must be
	// bound for either read to answer at all (an unbound chosen list fails
	// closed, including beneath the negation), so the precondition is asserted
	// by checking BOTH directions with the same binding.
	chosenLandID := corpusObject(t, reg, g, "Island").ID
	otherLandID := corpusObject(t, reg, g, "Mountain").ID
	chosenCtx := SpecContext{You: 0, ChosenValid: true, Chosen: []state.Target{{Obj: chosenLandID}}}
	if MatchesObjectCtx(g, "Card.ChosenCard", g.Obj(chosenLandID), chosenCtx) ==
		MatchesObjectCtx(g, "Card.ChosenCard", g.Obj(otherLandID), chosenCtx) {
		t.Fatalf("precondition: the chosen land and the other land must differ under ChosenCard")
	}
	if !MatchesObjectCtx(g, "Land.nonChosenCard", g.Obj(otherLandID), chosenCtx) {
		t.Errorf("Land.nonChosenCard must match a land that is not chosen (Mountain)")
	}
	if MatchesObjectCtx(g, "Land.nonChosenCard", g.Obj(chosenLandID), chosenCtx) {
		t.Errorf("Land.nonChosenCard must not match the chosen land (Island)")
	}

	// Leaf 3: nonCopiedSpell excludes a CR 707 copy of a spell. The Heron
	// Moon reads it on a card leaving the battlefield, but the predicate is
	// about the copy bit wherever the object is live; use a stack object, the
	// zone where a spell copy is not the CR 707.10h ephemeral reject.
	spellID := corpusObject(t, reg, g, "Lightning Bolt").ID
	g.Obj(spellID).Zone = state.ZStack
	copySpellID := corpusObject(t, reg, g, "Lightning Bolt").ID
	g.Obj(copySpellID).Zone = state.ZStack
	g.Obj(copySpellID).IsCopy = true
	if g.Obj(spellID).IsCopy {
		t.Fatalf("precondition: the plain spell must not be a copy")
	}
	if !g.Obj(copySpellID).IsCopy || g.Obj(copySpellID).Zone != state.ZStack {
		t.Fatalf("precondition: the copied spell must be a live stack copy")
	}
	if !MatchesObjectCtx(g, "Card.nonCopiedSpell", g.Obj(spellID), SpecContext{You: 0}) {
		t.Errorf("Card.nonCopiedSpell must match a spell that is not a copy")
	}
	if MatchesObjectCtx(g, "Card.nonCopiedSpell", g.Obj(copySpellID), SpecContext{You: 0}) {
		t.Errorf("Card.nonCopiedSpell must not match a copy of a spell")
	}

	// The matcher and UnknownPredicates must agree: each token is recognised.
	for _, spec := range []string{"Card.nonColorless", "Land.nonChosenCard", "Card.nonCopiedSpell"} {
		if un := UnknownPredicates(spec); len(un) != 0 {
			t.Errorf("UnknownPredicates(%q) = %v, want none (the matcher recognises it)", spec, un)
		}
	}
}
