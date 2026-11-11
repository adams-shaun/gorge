package kgen

import (
	"strings"
	"testing"
)

// TestPreventNextLowers: the one-recipient prevent-the-next-N shapes lower to
// one PreventNext instruction (the amount, then the recipient).
func TestPreventNextLowers(t *testing.T) {
	for _, tc := range []struct{ name, line string }{
		{"any target", "A:SP$ PreventDamage | ValidTgts$ Any | Amount$ 3 | SpellDescription$ Probe."},
		{"creature target", "A:SP$ PreventDamage | ValidTgts$ Creature | Amount$ 3 | SpellDescription$ Probe."},
		{"you", "A:SP$ PreventDamage | Defined$ You | Amount$ 2 | SpellDescription$ Probe."},
	} {
		src := "Name:Probe Salve\nManaCost:W\nTypes:Instant\n" + tc.line + "\nOracle:Probe.\n"
		c := lowerScriptCard(t, "Probe Salve", src)
		if len(c.Refusals) != 0 {
			t.Errorf("%s: refused: %v", tc.name, c.Refusals)
			continue
		}
		n := 0
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "PreventNext" {
					n++
				}
			}
		}
		if n != 1 {
			t.Errorf("%s: %d PreventNext instructions, want 1", tc.name, n)
		}
	}
}

// TestPreventNextRefusals: each shape the shield cannot model refuses with its
// own reason, in the api:PreventDamage class.
func TestPreventNextRefusals(t *testing.T) {
	for _, tc := range []struct{ line, want string }{
		{"A:SP$ PreventDamage | ValidTgts$ Any | Amount$ 3 | PreventionSubAbility$ Rider | ShieldEffectTarget$ ParentTarget | SpellDescription$ Probe.", "PreventionSubAbility: a rider"},
		{"A:SP$ PreventDamage | ValidTgts$ Creature | Amount$ 1 | Radiance$ True | SpellDescription$ Probe.", "Radiance"},
		{"A:SP$ PreventDamage | Defined$ You | Amount$ 1 | CombatDamage$ True | SpellDescription$ Probe.", "CombatDamage: a combat-damage-only shield"},
		{"A:SP$ PreventDamage | Defined$ Valid Creature & Player | Amount$ 1 | SpellDescription$ Probe.", "a set of recipients"},
	} {
		src := "Name:Probe Salve\nManaCost:W\nTypes:Instant\n" + tc.line + "\nOracle:Probe.\n"
		c := lowerScriptCard(t, "Probe Salve", src)
		if len(c.Refusals) == 0 {
			t.Errorf("%s: lowered", tc.line)
			continue
		}
		if got := c.Refusals[0].Reason; !strings.Contains(got, "api PreventDamage: ") || !strings.Contains(got, tc.want) {
			t.Errorf("%s: reason %q, want api PreventDamage: ...%s", tc.line, got, tc.want)
		}
	}
}
