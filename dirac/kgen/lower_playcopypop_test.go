package kgen

import (
	"strings"
	"testing"
)

// CopyCard$ True lowers over Defined$ Remembered behind the Exile that
// remembered its card (lower_playcopypop.go): Narset, Enlightened Exile and
// Roving Actuator carry no refusal and a CastFree with the Copy flag. Every
// other carrier stays refused by a Play population reason that names it.
func TestPlayCopyCardShapes(t *testing.T) {
	lowered := map[string]bool{"Narset, Enlightened Exile": true, "Roving Actuator": true}
	refusedBy := map[string]string{
		"Massimo, the Magician":   `Defined$ "Imprinted"`,
		"Baron Helmut Zemo":       "ExileFromGrave",
		"Arcane Bombardment":      "OrderDuplicates",
		"God-Eternal Kefnet":      "ForReveal",
		"Founding the Third Path": "Defined$ Remembered paid copy",
		"Capricious Hellraiser":   "random graveyard exile",
	}
	var names []string
	for n := range lowered {
		names = append(names, n)
	}
	for n := range refusedBy {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		seen++
		if lowered[c.Name] {
			if len(c.Refusals) != 0 {
				t.Errorf("%s refusals = %v, want none", c.Name, c.Refusals)
			}
			found := false
			for _, a := range c.Abilities {
				for _, in := range a.Prog {
					if in.Op == "CastFree" && strings.Contains(in.CS, "Copy") {
						found = true
					}
				}
			}
			if !found {
				t.Errorf("%s: no CastFree carries the Copy flag", c.Name)
			}
			continue
		}
		var all []string
		for _, r := range c.Refusals {
			all = append(all, r.Reason)
		}
		if got := strings.Join(all, "; "); !strings.Contains(got, refusedBy[c.Name]) {
			t.Errorf("%s refusals = %q, want one naming %q", c.Name, got, refusedBy[c.Name])
		}
	}
	if seen != len(names) {
		t.Fatalf("lowered %d carriers, want %d", seen, len(names))
	}
}

// A paid Defined$ Remembered copy and a Defined$ Remembered Play not chained
// behind an Exile stay refused by name.
func TestPlayCopyCardDefinedRefuses(t *testing.T) {
	src := func(play string) string {
		return strings.Join([]string{
			"Name:Probe",
			"ManaCost:1 R",
			"Types:Creature Human",
			"PT:1/1",
			"A:AB$ Play | Cost$ T | " + play + " | SpellDescription$ Probe.",
		}, "\n")
	}
	got := abReasons(abLower(t, src("Defined$ Remembered | ValidSA$ Spell | CopyCard$ True | WithoutManaCost$ True | Optional$ True"), false))
	if !strings.Contains(got, "not behind the Exile that remembered it") {
		t.Fatalf("refusals = %q, want the unchained refusal", got)
	}
}
