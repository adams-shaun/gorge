package kgen

import (
	"strings"
	"testing"
)

// TestMetalcraftGatedMana: a card's lone plain {T} mana ability gated on
// Activation$ Metalcraft (Mox Opal, CR 207.2c, 602.5b) lowers as a gated mana
// source. The same gate on a non-mana ability, or on one of two mana
// abilities, refuses by name: only the lone-ability shape is lockstep-verified
// (dirac/oracle TestMoxOpalLockstep).
func TestMetalcraftGatedMana(t *testing.T) {
	const head = "Name:Opal Probe\nManaCost:0\nTypes:Artifact\n"
	const gated = "A:AB$ Mana | Cost$ T | Produced$ Any | Activation$ Metalcraft | SpellDescription$ Probe.\n"
	if _, rs := kwardpowBuild(t, "Opal Probe", head+gated); len(rs) != 0 {
		t.Fatalf("a lone metalcraft-gated mana ability refused: %v", rs)
	}
	for _, c := range []struct{ name, src, want string }{
		{"a second mana ability", head + "A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Plain.\n" + gated, "metalcraft-gated mana ability beside another mana ability"},
		{"a non-mana ability", head + "A:AB$ Draw | Cost$ T | NumCards$ 1 | Activation$ Metalcraft | SpellDescription$ Probe.\n", "outside a mana ability"},
	} {
		_, rs := kwardpowBuild(t, "Opal Probe", c.src)
		var all []string
		for _, r := range rs {
			all = append(all, r.Reason)
		}
		if !strings.Contains(strings.Join(all, "; "), c.want) {
			t.Errorf("%s: refusals %q, want %q", c.name, all, c.want)
		}
	}
	for _, c := range lowerNamed(t, []string{"Mox Opal"}) {
		if len(c.Refusals) > 0 {
			t.Errorf("%s: refused: %v", c.Name, c.Refusals)
		}
	}
}
