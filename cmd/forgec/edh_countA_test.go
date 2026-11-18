package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHCountHeadsASupported pins the count-head class ticket edh-countA:
// these imported-Commander cards each read a Count$<Head> the count
// evaluator models, so cards.Registry.Unsupported must report no missing
// primitive for any of them (make report / forgec edhcover predicate,
// with the ETB copy-shape gate set).
func TestEDHCountHeadsASupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	cards := []string{
		"Boss's Chauffeur",           // count:LastStateBattlefield
		"Aeve, Progenitor Ooze",      // count:LastStateBattlefield
		"From Under the Floorboards", // count:Madness
		"Avacyn's Judgment",          // count:Madness
		"Notorious Throng",           // count:BloodthirstAmount
		"Sarevok's Tome",             // count:Initiative
		"Ephara, God of the Polis",   // count:LastTurnEntered
		"Impact Resonance",           // count:SingleMaxDamageThisTurn
		"Indulge",                    // count:NumCombatDamageThisTurn
	}
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	for _, name := range cards {
		c, ok := r.Lookup(name)
		if !ok {
			t.Fatalf("corpus missing %q", name)
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
