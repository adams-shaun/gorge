package kgen

import (
	"strings"
	"testing"
)

// TestFilterFiveNegatedSubtypes: a clause may negate five subtypes, as Summon:
// Leviathan's first chapter does; a sixth refuses. The filter is one clause, so
// the first negation rides in NotSub and the other four in NotSubs.
func TestFilterFiveNegatedSubtypes(t *testing.T) {
	known := func(string) bool { return true }
	c, err := parseClause("Creature.nonKraken+nonLeviathan+nonMerfolk+nonOctopus+nonSerpent", known)
	if err != nil {
		t.Fatalf("five negations: %v", err)
	}
	if c.NotSubtype != "Kraken" || c.MoreNotSubtype != [4]string{"Leviathan", "Merfolk", "Octopus", "Serpent"} {
		t.Fatalf("negations = %q %v", c.NotSubtype, c.MoreNotSubtype)
	}
	if _, err := parseClause("Creature.nonKraken+nonLeviathan+nonMerfolk+nonOctopus+nonSerpent+nonHuman", known); err == nil ||
		!strings.Contains(err.Error(), "six negated subtypes") {
		t.Fatalf("six negations: got %v", err)
	}
	checkForms(t, []formCase{
		{card: "Summon: Leviathan", form: "five negated subtypes in one clause",
			lir: "notsub=Kraken notsub=Leviathan notsub=Merfolk notsub=Octopus notsub=Serpent"},
	})
}
