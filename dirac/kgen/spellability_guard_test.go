package kgen

import (
	"slices"
	"strings"
	"testing"
)

// TestValidSAGuardClasses: the spell-ability filters kgen refuses because the
// kernel has no word for them yet (T13c's FwSA* family) carry an explicit
// guard class, so the census counts the guard instead of a bare other: row
// (the T12 design: "refuse them with an explicit guard class so T01 counts
// them"). The class is tiered M5 with the filter-word context guard.
func TestValidSAGuardClasses(t *testing.T) {
	for _, tc := range []struct {
		what, src, reason, class string
	}{
		{
			what: "a SpellCast trigger's ValidSA$",
			src: strings.Join([]string{
				"Name:Probe Spent",
				"ManaCost:1 U",
				"Types:Creature",
				"PT:1/1",
				"T:Mode$ SpellCast | ValidSA$ Spell.ManaSpent GTX | TriggerZones$ Battlefield | Execute$ ProbeDraw | TriggerDescription$ Probe.",
				"SVar:ProbeDraw:DB$ Draw | NumCards$ 2",
				"Oracle:Probe.",
			}, "\n"),
			reason: `spell-ability filter ValidSA "Spell.ManaSpent GTX"`,
			class:  `other:spell-ability filter ValidSA "_"`,
		},
		{
			what: "an AbilityCast trigger's ValidSA$",
			src: strings.Join([]string{
				"Name:Probe Bracers",
				"ManaCost:3",
				"Types:Artifact",
				"T:Mode$ AbilityCast | ValidSA$ SpellAbility.!ManaAbility | TriggerZones$ Battlefield | Execute$ ProbeDraw | TriggerDescription$ Probe.",
				"SVar:ProbeDraw:DB$ Draw | NumCards$ 2",
				"Oracle:Probe.",
			}, "\n"),
			reason: `AbilityCast spell-ability filter ValidSA$ "SpellAbility.!ManaAbility"`,
			class:  `other:AbilityCast spell-ability filter ValidSA$ "_"`,
		},
		{
			what: "a static's unread ValidSA$",
			src: strings.Join([]string{
				"Name:Probe Phoenix",
				"ManaCost:2 R",
				"Types:Creature Phoenix",
				"PT:2/2",
				"S:Mode$ Continuous | Affected$ Card.Self | MayPlay$ True | ValidSA$ Spell.Blitz | AffectedZone$ Graveyard | EffectZone$ Graveyard | Description$ Probe.",
				"Oracle:Probe.",
			}, "\n"),
			reason: `static spell-ability filter ValidSA Continuous "Spell.Blitz"`,
			class:  `other:static spell-ability filter ValidSA Continuous "_"`,
		},
	} {
		c := lowerScriptCard(t, "Probe", tc.src)
		if len(c.Refusals) == 0 {
			t.Errorf("%s lowered: want the spell-ability filter guard", tc.what)
			continue
		}
		got := c.Refusals[0].Reason
		if !strings.Contains(got, tc.reason) {
			t.Errorf("%s: reason %q, want it to name %q", tc.what, got, tc.reason)
		}
		if cl := refusalClasses(got); !slices.Equal(cl, []string{tc.class}) {
			t.Errorf("%s: classes %v, want [%s]", tc.what, cl, tc.class)
		}
	}
	for _, tc := range []struct{ class, tier string }{
		{"other:filter word FwPowerCmp in perm", "M5"},
		{"other:filter word FwCountersCmp in card", "M5"},
		{`other:spell-ability filter ValidSA "_"`, "M5"},
		{`other:static spell-ability filter ValidSA Continuous "_"`, "M5"},
		{`other:AbilityCast spell-ability filter ValidSA$ "_"`, "M5"},
	} {
		if got := classTier(tc.class); got != tc.tier {
			t.Errorf("classTier(%q) = %q, want %q", tc.class, got, tc.tier)
		}
	}
}
