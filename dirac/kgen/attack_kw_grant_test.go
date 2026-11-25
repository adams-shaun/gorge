package kgen

import (
	"strings"
	"testing"
)

// TestGrantedAttackKeywordsLower: a Continuous static's AddKeyword$
// Annihilator:N or Battle cry, and a Pump KW$ of either, lower to a latent
// Attacks Self trigger granted per instance (lower_attackkwgrant.go); a
// non-literal count (Ulamog, the Defiler's Annihilator:X) refuses by name.
func TestGrantedAttackKeywordsLower(t *testing.T) {
	const creature = "Name:Probe\nManaCost:1 W\nTypes:Creature Human\nPT:1/1\n"
	for _, c := range []struct{ name, src, latent string }{
		{"static annihilator", creature + "S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Trample & Annihilator:2\n", "GrantedAnnihilator:2"},
		{"static battle cry", creature + "S:Mode$ Continuous | Affected$ Creature.YouCtrl | AddKeyword$ Battle cry\n", "GrantedBattlecry"},
		{"pump battle cry", creature + "A:AB$ Pump | Cost$ 0 | Defined$ Self | KW$ Battle cry | SpellDescription$ x\n", "GrantedBattlecry"},
		{"pumpall annihilator", creature + "A:AB$ PumpAll | Cost$ 0 | ValidCards$ Creature.YouCtrl | KW$ Annihilator:1 & Indestructible | SpellDescription$ x\n", "GrantedAnnihilator:1"},
	} {
		lc := horsemanshipGrantLower(t, c.src)
		if len(lc.Refusals) != 0 {
			t.Errorf("%s refused: %v", c.name, lc.Refusals)
			continue
		}
		found := false
		for _, a := range lc.Abilities {
			if a.Latent && a.Name == c.latent {
				found = true
			}
		}
		if !found {
			t.Errorf("%s: no latent %q ability", c.name, c.latent)
		}
	}
	x := horsemanshipGrantLower(t, creature+"S:Mode$ Continuous | Affected$ Card.Self | AddKeyword$ Annihilator:X\nSVar:X:Count$CardCounters.P1P1\n")
	var got []string
	for _, r := range x.Refusals {
		got = append(got, r.Reason)
	}
	if !strings.Contains(strings.Join(got, "; "), `granted Annihilator "Annihilator:X"`) {
		t.Errorf("Annihilator:X refusals %q, want one naming the count", got)
	}
}
