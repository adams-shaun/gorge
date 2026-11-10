package kgen

import (
	"strings"
	"testing"
)

const (
	crownScript = "Name:Copy Probe\nManaCost:4\nTypes:Artifact Equipment\nK:Equip:2\n" +
		"R:Event$ CreateToken | ActiveZones$ Battlefield | IsPresent$ Card.Self+AttachedTo Creature | CheckSVar$ X | SVarCompare$ EQ0 | ValidPlayer$ You | Optional$ True | Layer$ Copy | ReplaceWith$ DBCopy | Description$ x\n" +
		"SVar:DBCopy:DB$ ReplaceToken | Type$ ReplaceToken | ValidChoices$ Card.EquippedBy | TokenScript$ Chosen\n" +
		"SVar:X:Count$ThisTurnEntered_Battlefield_Card.tokenCreated+YouOwn\n"
)

// TestLowerReplTokenCopyFirstturn: the optional copy-instead CreateToken
// replacement of an Equipment or Aura gated on no token of yours having
// entered this turn lowers (Mirrormind Crown, Moonlit Meditation: a
// CopyFirstToken replacement whose Cond is the gate), and each variant that
// leaves the shape still refuses by name.
func TestLowerReplTokenCopyFirstturn(t *testing.T) {
	for _, name := range []string{"Mirrormind Crown", "Moonlit Meditation"} {
		lc := lowerCorpusCard(t, name)
		for _, rf := range lc.Refusals {
			t.Errorf("%s refused: %s", name, rf.Reason)
		}
		if len(lc.Repls) != 1 || lc.Repls[0].Kind != "CopyFirstToken" || lc.Repls[0].Cond.Kind != cValue || lc.Repls[0].Cond.Op != opEQ {
			t.Errorf("%s lowered %+v, want one CopyFirstToken replacement gated by a value compare", name, lc.Repls)
		}
	}
	rep := func(old, new string) string {
		if !strings.Contains(crownScript, old) {
			t.Fatalf("variant text %q not in the script", old)
		}
		return strings.Replace(crownScript, old, new, 1)
	}
	for _, c := range []struct{ name, script, want string }{
		{"the shape", crownScript, ""},
		{"an Aura's host", strings.Replace(rep("Card.EquippedBy", "Card.EnchantedBy"), "IsPresent$ Card.Self+AttachedTo Creature | ", "", 1), ""},
		{"opponent's tokens", rep("ValidPlayer$ You", "ValidPlayer$ Opponent"), "CreateToken copy ValidPlayer"},
		{"every creation", rep("ValidPlayer$ You | ", ""), "CreateToken copy ValidPlayer"},
		{"not a may", rep("Optional$ True | ", ""), "CreateToken copy replacement Layer/Optional"},
		{"no copy layer", rep("Layer$ Copy | ", ""), "CreateToken copy replacement Layer/Optional"},
		{"tokens it controls", rep("tokenCreated+YouOwn", "tokenCreated+YouCtrl"), "CreateToken copy gate"},
		{"any permanent entered", rep("Card.tokenCreated+YouOwn", "Card.YouOwn"), "CreateToken copy gate"},
		{"the second time", rep("SVarCompare$ EQ0", "SVarCompare$ EQ1"), "CreateToken copy gate"},
		{"no gate", rep("CheckSVar$ X | SVarCompare$ EQ0 | ", ""), "CreateToken copy gate"},
		{"another presence test", rep("Card.Self+AttachedTo Creature", "Creature.YouCtrl"), "CreateToken copy IsPresent"},
		{"a presence test on an Aura", strings.Replace(rep("Card.EquippedBy", "Card.EnchantedBy"), "Card.Self+AttachedTo Creature", "Card.Self+AttachedTo Artifact", 1), "CreateToken copy IsPresent"},
		{"a rider on the body", rep("TokenScript$ Chosen", "TokenScript$ Chosen | RememberTokens$ True"), "CreateToken copy replacement shape"},
		{"a choice among other creatures", rep("Card.EquippedBy", "Creature.Other"), "CreateToken Layer"}, // Esix's shape stays refused
	} {
		t.Run(c.name, func(t *testing.T) {
			got := strings.Join(lowerScript(t, "Copy Probe", c.script), "; ")
			if c.want == "" {
				if got != "" {
					t.Fatalf("refused: %s", got)
				}
				return
			}
			if !strings.Contains(got, c.want) {
				t.Fatalf("refusals %q, want one naming %q", got, c.want)
			}
		})
	}
}
