package kgen

import (
	"slices"
	"strings"
	"testing"
)

// spellabilityProbe is a creature carrying one SpellCast trigger over head.
func spellabilityProbe(head, extra string) string {
	return strings.Join([]string{
		"Name:Probe Cast",
		"ManaCost:1 U",
		"Types:Creature",
		"PT:1/1",
		"T:Mode$ SpellCast | ValidSA$ " + head + extra + " | TriggerZones$ Battlefield | Execute$ ProbeDraw | TriggerDescription$ Probe.",
		"SVar:ProbeDraw:DB$ Draw | NumCards$ 1",
		"Oracle:Probe.",
	}, "\n")
}

// TestFilterWordsSpellability: the SpellCast ValidSA$ heads the three kernel
// words answer lower; the heads that stay refused keep the guard class and
// carry their reason word.
func TestFilterWordsSpellability(t *testing.T) {
	for _, head := range []string{
		"Spell.Modal",
		"Spell.singleTarget",
		"Instant.singleTarget,Sorcery.singleTarget",
		"Spell.Teamwork",
	} {
		c := lowerScriptCard(t, "Probe Cast", spellabilityProbe(head, " | ValidActivatingPlayer$ You"))
		if len(c.Refusals) != 0 {
			t.Errorf("ValidSA$ %s refused: %v", head, c.Refusals[0].Reason)
		}
	}
	// the word reads the cast's own provenance: another ValidCard$ filter
	// beside it would need an AND, so it stays refused
	c := lowerScriptCard(t, "Probe Cast", spellabilityProbe("Spell.Modal", " | ValidCard$ Card.Red"))
	if len(c.Refusals) == 0 {
		t.Errorf("ValidSA$ Spell.Modal beside ValidCard$ lowered")
	}
	for _, tc := range []struct{ head, why string }{
		{"Spell.ManaSpentBy Player.Other", "mana-provenance"},
		{"Spell.ManaSpent GTX", "mana-provenance"},
		{"Spell.ManaFromCreature_3", "mana-provenance"},
		{"Spell.Kicked", "gorge-unmodelled"},
		{"Spell.Warp", "gorge-unmodelled"},
		{"Spell.numTargets GE1", "gorge-unmodelled"},
	} {
		c := lowerScriptCard(t, "Probe Cast", spellabilityProbe(tc.head, ""))
		if len(c.Refusals) == 0 {
			t.Errorf("ValidSA$ %s lowered", tc.head)
			continue
		}
		got := c.Refusals[0].Reason
		if !strings.Contains(got, `spell-ability filter ValidSA "`+tc.head+`"`) || !strings.Contains(got, tc.why) {
			t.Errorf("ValidSA$ %s: reason %q, want the head and %q", tc.head, got, tc.why)
		}
		if cl := refusalClasses(got); !slices.Equal(cl, []string{`other:spell-ability filter ValidSA "_"`}) {
			t.Errorf("ValidSA$ %s: classes %v, want the guard class", tc.head, cl)
		}
	}
}
