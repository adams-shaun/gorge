package kgen

import (
	"strings"
	"testing"
)

// TestHostedPlayShapesLower is the lowering half of the hosted-Commander Play
// slice: each of the four cards must lower (they were refused by name before
// this round) and carry its shape in the IR -- Conduit of Worlds' paid
// remembered cast, Rashmi and Ragavan's ImprintPlayed$ flag, Scarlet Witch's
// and West Coast Expansion's Spell.Hero filter.
func TestHostedPlayShapesLower(t *testing.T) {
	names := []string{
		"Conduit of Worlds", "Rashmi and Ragavan", "Scarlet Witch, Chaotic Avenger", "West Coast Expansion",
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		seen++
		switch c.Name {
		case "Conduit of Worlds":
			if len(c.Refusals) != 0 {
				t.Errorf("%s refusals = %v, want none", c.Name, c.Refusals)
			}
			if !castFreeHas(c, func(in lInstr) bool {
				return strings.Contains(in.CS, "Paid") && strings.Contains(in.CS, "Remember")
			}) {
				t.Errorf("%s: no paid, RememberPlayed$ CastFree", c.Name)
			}
		case "Rashmi and Ragavan":
			if len(c.Refusals) != 0 {
				t.Errorf("%s refusals = %v, want none", c.Name, c.Refusals)
			}
			if !castFreeHas(c, func(in lInstr) bool { return strings.Contains(in.CS, "Imprint") }) {
				t.Errorf("%s: no ImprintPlayed$ CastFree", c.Name)
			}
		case "Scarlet Witch, Chaotic Avenger", "West Coast Expansion":
			if len(c.Refusals) != 0 {
				t.Errorf("%s refusals = %v, want none", c.Name, c.Refusals)
			}
			if !castFreeHas(c, func(in lInstr) bool { return filterHasHero(in.F) }) {
				t.Errorf("%s: the Play filter has no Hero clause", c.Name)
			}
		}
	}
	if seen != len(names) {
		t.Fatalf("lowered %d cards, want %d", seen, len(names))
	}
}

// castFreeHas reports whether any ability program holds a CastFree satisfying f.
func castFreeHas(c *lCard, f func(lInstr) bool) bool {
	for i := range c.Abilities {
		for _, in := range c.Abilities[i].Prog {
			if in.Op == "CastFree" && f(in) {
				return true
			}
		}
	}
	return false
}

// filterHasHero reports a clause carrying the Hero subtype (the ValidSA$
// Spell.Hero arm's filter clause).
func filterHasHero(f *lFilter) bool {
	if f == nil {
		return false
	}
	for _, c := range f.Clauses {
		if c.Subtype == "Hero" {
			return true
		}
	}
	return false
}
