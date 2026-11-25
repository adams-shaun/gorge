package kgen

import (
	"strings"
	"testing"
)

// dblAnySrcCarriers are the slice's DamageDone doubler cards, the shapes this
// slice lowers: the doubler whose ValidSource$ names no controller (Card,Emblem
// — any source's, any controller's: Dictate of the Twin Gods, Furnace of Rath,
// Fiendish Duo), the doubler's own Thrice (City on Fire, Fiery Emancipation),
// and the ValidTarget$ You / Card.Self words (Goldnight
// Castigator doubles the damage to its controller and to itself). Each lowers
// from gorge's DB$ ReplaceEffect | VarName$ DamageAmount body to a
// ReplDoubleDamageToOpps (dirac/repl_multipliers.go).
var dblAnySrcCarriers = []string{
	"City on Fire", "Dictate of the Twin Gods", "Fiendish Duo",
	"Fiery Emancipation", "Furnace of Rath", "Goldnight Castigator",
}

func TestReplDrawDamageSlice(t *testing.T) {
	for _, name := range dblAnySrcCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// The slice's other carriers keep the refusals the ticket names: the draw
// replacements (plain, optional or choice) and the two DamageDone replacement
// shapes whose body asks a player stay refused until their own slices lower
// them.
func TestReplDrawDamageRestRefused(t *testing.T) {
	for _, w := range []struct{ name, class string }{
		{"Abundance", "Draw replacement with an optional or choice body"},
		{"Archmage Ascension", "Draw replacement with an optional or choice body"},
		{"Asmodeus the Archfiend", "Draw replacement"},
		{"Blood Scrivener", "Draw replacement"},
		{"Dralnu, Lich Lord", "DamageDone replacement shape"},
		{"Enduring Renewal", "Draw replacement"},
		{"Eruth, Tormented Prophet", "Draw replacement"},
		{"Hullbreacher", "Draw replacement"},
		{"Notion Thief", "Draw replacement"},
		{"Obstinate Familiar", "Draw replacement with an optional or choice body"},
		{"Out of the Tombs", "Draw replacement"},
		{"Pavel Chekov, First Posting", "Draw replacement with an optional or choice body"},
		{"Phyrexian Vindicator", "DamageDone replacement shape"},
		{"Possessed Portal", "Draw replacement"},
		{"Pursuit of Knowledge", "Draw replacement with an optional or choice body"},
		{"Sages of the Anima", "Draw replacement"},
		{"Tomorrow, Azami's Familiar", "Draw replacement"},
		{"Underrealm Lich", "Draw replacement"},
		{"Unpredictable Cyclone", "Draw replacement"},
		{"Breathstealer's Crypt", "Draw replacement"},
		{"Chains of Mephistopheles", "Draw replacement"},
		{"Forbidden Crypt", "Draw replacement"},
	} {
		lc := lowerNamedCard(t, w.name)
		if len(lc.Refusals) != 1 {
			t.Errorf("%s refusals = %v, want exactly one", w.name, lc.Refusals)
			continue
		}
		if got := lc.Refusals[0].Reason; !strings.Contains(got, w.class) {
			t.Errorf("%s refusal %q, want it to name %q", w.name, got, w.class)
		}
	}
}
