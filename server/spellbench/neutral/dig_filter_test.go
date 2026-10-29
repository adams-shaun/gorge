package neutral

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestDigEligibleFailsClosedOnUnmodelledPredicates pins digAlt's contract: a
// Card.<word> predicate is translated only when <word> is a known card type.
// Any other word (a named card, a chosen type, the historic supertype, a mana
// value comparison) must report the filter as unknown so the neutral rebuild
// leaves the decision opaque rather than marking every card ineligible (or,
// negated, eligible).
func TestDigEligibleFailsClosedOnUnmodelledPredicates(t *testing.T) {
	f := &cards.Face{Types: []string{"Instant"}}
	// Precondition: the face is where the filter looks, and its type line has
	// the word the positive controls below rely on.
	if !slices.Contains(f.Types, "Instant") {
		t.Fatalf("precondition: face types %v do not contain Instant", f.Types)
	}

	// Positive controls: modelled type words still classify, and a real type
	// the card lacks is a known mismatch rather than an unknown predicate.
	if match, known := digEligible(f, "Card.nonCreature+nonLand"); !known || !match {
		t.Errorf("digEligible(Card.nonCreature+nonLand) = (%v, %v), want (true, true)", match, known)
	}
	if match, known := digEligible(f, "Card.Creature"); !known || match {
		t.Errorf("digEligible(Card.Creature) = (%v, %v), want (false, true)", match, known)
	}
	if match, known := digEligible(f, "Card.instant"); !known || !match {
		t.Errorf("digEligible(Card.instant) = (%v, %v), want (true, true)", match, known)
	}

	for _, spec := range []string{
		"Card.NamedCard",
		"Card.ChosenType",
		"Card.Historic",
		"Card.nonHistoric",
		"Card.cmcLE3",
		"Card.powerGE2",
		"Card.White",
		"Card.nonRed",
	} {
		match, known := digEligible(f, spec)
		if known || match {
			t.Errorf("digEligible(%q) = (%v, %v), want opaque (false, false)", spec, match, known)
		}
	}
}
