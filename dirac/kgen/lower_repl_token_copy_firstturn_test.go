package kgen

import (
	"slices"
	"strings"
	"testing"
)

// copyProbe assembles an Equipment probe whose replacement is the shape under
// test: the parameter fields of the R: line, the body's fields and the SVar X
// (the card's own text is not copied: this is the shape).
func copyProbe(repl, body []string, x string) string {
	return "Name:Copy Probe\nManaCost:4\nTypes:Artifact Equipment\nK:Equip:2\n" +
		"R:" + strings.Join(repl, " | ") + "\n" +
		"SVar:DBCopy:" + strings.Join(body, " | ") + "\n" +
		"SVar:X:" + x + "\n"
}

var (
	probeRepl = []string{"Event$ CreateToken", "ActiveZones$ Battlefield", "ValidPlayer$ You", "Optional$ True", "Layer$ Copy",
		"CheckSVar$ X", "SVarCompare$ EQ0", "IsPresent$ Card.Self+AttachedTo Creature", "ReplaceWith$ DBCopy", "Description$ probe"}
	probeBody = []string{"DB$ ReplaceToken", "Type$ ReplaceToken", "ValidChoices$ Card.EquippedBy", "TokenScript$ Chosen"}
	probeGate = "Count$ThisTurnEntered_Battlefield_Card." + "tokenCreated+YouOwn"
)

// variant is the probe with one field of the replacement (or body) replaced by
// new ("" drops it) and, when gate is set, its own SVar X.
func variant(old, new, gate string) string {
	sub := func(fs []string) []string {
		var out []string
		for _, f := range fs {
			switch {
			case f != old:
				out = append(out, f)
			case new != "":
				out = append(out, new)
			}
		}
		return out
	}
	if gate == "" {
		gate = probeGate
	}
	return copyProbe(sub(probeRepl), sub(probeBody), gate)
}

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
	auraRepl := []string{"Event$ CreateToken", "ActiveZones$ Battlefield", "ValidPlayer$ You", "Optional$ True", "Layer$ Copy",
		"CheckSVar$ X", "SVarCompare$ EQ0", "ReplaceWith$ DBCopy", "Description$ probe"}
	auraBody := []string{"DB$ ReplaceToken", "Type$ ReplaceToken", "ValidChoices$ Card.EnchantedBy", "TokenScript$ Chosen"}
	for _, c := range []struct{ name, script, want string }{
		{"the shape", copyProbe(probeRepl, probeBody, probeGate), ""},
		{"an Aura's host", copyProbe(auraRepl, auraBody, probeGate), ""},
		{"opponent's tokens", variant("ValidPlayer$ You", "ValidPlayer$ Opponent", ""), "CreateToken copy ValidPlayer"},
		{"every creation", variant("ValidPlayer$ You", "", ""), "CreateToken copy ValidPlayer"},
		{"not a may", variant("Optional$ True", "", ""), "CreateToken copy replacement Layer/Optional"},
		{"no copy layer", variant("Layer$ Copy", "", ""), "CreateToken copy replacement Layer/Optional"},
		{"tokens it controls", variant("", "", "Count$ThisTurnEntered_Battlefield_Card.token+YouCtrl"), "CreateToken copy gate"},
		{"any permanent entered", variant("", "", "Count$ThisTurnEntered_Battlefield_Card.YouOwn"), "CreateToken copy gate"},
		{"the second time", variant("SVarCompare$ EQ0", "SVarCompare$ EQ1", ""), "CreateToken copy gate"},
		{"no gate", copyProbe(slices.DeleteFunc(slices.Clone(auraRepl), func(f string) bool { return strings.HasPrefix(f, "CheckSVar") || strings.HasPrefix(f, "SVarCompare") }), auraBody, probeGate), "CreateToken copy gate"},
		{"another presence test", variant("IsPresent$ Card.Self+AttachedTo Creature", "IsPresent$ Creature.YouCtrl", ""), "CreateToken copy IsPresent"},
		{"a presence test on an Aura", copyProbe(append(append([]string(nil), auraRepl...), "IsPresent$ Card.Self+AttachedTo Creature"), auraBody, probeGate), "CreateToken copy IsPresent"},
		{"a rider on the body", copyProbe(probeRepl, append(append([]string(nil), probeBody...), "RememberTokens$ True"), probeGate), "CreateToken copy replacement shape"},
		{"a choice among other creatures", variant("ValidChoices$ Card.EquippedBy", "ValidChoices$ Creature.Other", ""), "CreateToken Layer"}, // Esix's shape stays refused
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
