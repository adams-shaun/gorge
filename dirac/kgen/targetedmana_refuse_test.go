package kgen

import (
	"strings"
	"testing"
)

// TestTargetedManaHeadRefuses: a head AB$ Mana carrying ValidTgts$ is not a
// CR 605.1a mana ability in gorge (it resolves on the stack and adds to the
// target), so it refuses with that reason first, for any source type; an
// untargeted head still lowers.
func TestTargetedManaHeadRefuses(t *testing.T) {
	const want = "AB$ Mana with a target"
	probes := []struct {
		name, src string
		abilities int
		refuses   bool
	}{
		{"Lotus Probe", "Name:Lotus Probe\nManaCost:6\nTypes:Artifact\n" +
			"A:AB$ Mana | Cost$ T Sac<X/Artifact> | XMin$ 1 | Chooser$ You | ValidTgts$ Player | Produced$ Any | Amount$ Y | SpellDescription$ x\n" +
			"SVar:X:Count$xPaid\nSVar:Y:SVar$X/Times.3\n", 0, true},
		{"Creature Target Probe", "Name:Creature Target Probe\nManaCost:1 G\nTypes:Creature Elf\nPT:1/1\n" +
			"A:AB$ Mana | Cost$ T | ValidTgts$ Player | Produced$ C | SpellDescription$ x\n", 0, true},
		{"Plain Mana Probe", "Name:Plain Mana Probe\nManaCost:1 G\nTypes:Creature Elf\nPT:1/1\n" +
			"A:AB$ Mana | Cost$ T | Produced$ C | SpellDescription$ x\n", 1, false},
	}
	for _, p := range probes {
		def, rs := kwardpowBuild(t, p.name, p.src)
		all := kpaylifexReasons(rs)
		if len(def.Abilities) != p.abilities {
			t.Errorf("%s: %d abilities, want %d (refusals %q)", p.name, len(def.Abilities), p.abilities, all)
		}
		if p.refuses {
			if !strings.Contains(all, want) {
				t.Errorf("%s: refusals %q, want %q", p.name, all, want)
			}
			if strings.Contains(all, "outside a spell's or activated ability's own cost") {
				t.Errorf("%s: refusals %q name the misleading sacrifice reason", p.name, all)
			}
		} else if all != "" {
			t.Errorf("%s: refusals %q, want none", p.name, all)
		}
	}
}
