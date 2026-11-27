package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestMetalcraftGatedMana: a card's lone plain {T} mana ability gated on
// Activation$ Metalcraft (Mox Opal, CR 207.2c, 602.5b) lowers as a gated mana
// source. The same gate on a non-mana ability lowers too (Brotherhood Scribe;
// dirac/oracle TestDemoMetalcraftLockstep). One of two fixed mana abilities
// still refuses by name: only the lone fixed-ability shape is
// lockstep-verified (dirac/oracle TestMoxOpalLockstep); the evaluated pair
// beside a plain ability is lower_metalcraftdyn.go's.
func TestMetalcraftGatedMana(t *testing.T) {
	const head = "Name:Opal Probe\nManaCost:0\nTypes:Artifact\n"
	const gated = "A:AB$ Mana | Cost$ T | Produced$ Any | Activation$ Metalcraft | SpellDescription$ Probe.\n"
	if _, rs := kwardpowBuild(t, "Opal Probe", head+gated); len(rs) != 0 {
		t.Fatalf("a lone metalcraft-gated mana ability refused: %v", rs)
	}
	def, rs := kwardpowBuild(t, "Opal Probe", head+"A:AB$ Draw | Cost$ T | NumCards$ 1 | Activation$ Metalcraft | SpellDescription$ Probe.\n")
	if len(rs) != 0 {
		t.Fatalf("a non-mana metalcraft-gated ability refused: %v", rs)
	}
	if len(def.Abilities) != 1 || def.Abilities[0].Kind != kop.AbActivated {
		t.Fatalf("non-mana metalcraft-gated ability: %v, want one activated ability", def.Abilities)
	}
	for _, c := range []struct{ name, src, want string }{
		{"a second mana ability", head + "A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ Plain.\n" + gated, "metalcraft-gated mana ability beside another mana ability"},
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
