package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHCountHeadsBSupported pins the class of imported Commander decks a
// missing count-expression head keeps from full gorge support: each card
// below reads a Count$ head (or PlayerCount property) the count evaluator
// does not model, so cards.Registry.Unsupported counts it unplayable. The
// test fails listing the card's missing keys until every head is modelled
// with real behaviour behind it.
func TestEDHCountHeadsBSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	for _, name := range []string{
		"Seal of the Guildpact",
		"Tablet of the Guilds",
		"Adipose Offspring",
		"Fblthp, the Lost",
		"Reset",
		"Sage of Ancient Lore",
		"Archive Trap",
		"Call to Arms",
		"Embodiment of Agonies",
		"Psychic Battle",
		"Springjack Shepherd",
	} {
		c, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("corpus missing %q", name)
		}
		r.ShapeGate = etbCopyShapeGate
		m := r.Unsupported(c, sup)
		r.ShapeGate = nil
		if len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
