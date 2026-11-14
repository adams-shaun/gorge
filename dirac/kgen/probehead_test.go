package kgen

import (
	"strings"
	"testing"
)

// abBodyRows is the every-blocker census's refusals on probed bodies
// (a node ending " body"), one "node: reason" per line.
func abBodyRows(c *lCard) []string {
	var rows []string
	for _, r := range c.Refusals {
		if strings.HasSuffix(r.Node, " body") {
			rows = append(rows, r.Node+": "+r.Reason)
		}
	}
	return rows
}

// TestProbeBodySkipsActivationHead: probeBody reads a refused activated
// ability's body, not its head. The activation Cost$ is no resolution cost
// (the census's "resolution cost shape" spelling on every refused AB$ came
// from it), the activation keys the AB arm reads are not unread body params,
// and a mana ability's head (lowerMana's, never chain's) is not probed as a
// DB$ Mana; its SubAbility$ chain still is.
func TestProbeBodySkipsActivationHead(t *testing.T) {
	head := func(rest ...string) string {
		return strings.Join(append([]string{"Name:Probe", "ManaCost:G", "Types:Creature Elf", "PT:1/1"}, rest...), "\n")
	}
	cases := []struct {
		name string
		src  string
		want []string // substrings some body row carries
	}{
		{
			// Gilanra's shape: the head refuses (TriggersWhenSpent$ unread)
			name: "mana",
			src: head("A:AB$ Mana | Cost$ T | Produced$ G | TriggersWhenSpent$ TrigSpent | SpellDescription$ Add {G}.",
				"SVar:TrigSpent:Mode$ SpellCast | ValidCard$ Card.cmcGE6 | Execute$ TrigDraw",
				"SVar:TrigDraw:DB$ Draw"),
		},
		{
			// a mana ability refused before lowerMana (a targeted AB$ Mana):
			// its sub-chain is the probed body
			name: "mana-sub",
			src: head("A:AB$ Mana | Cost$ T | Produced$ G | ValidTgts$ Player | SubAbility$ DBDraw | SpellDescription$ Add {G}.",
				"SVar:DBDraw:DB$ Draw | Defined$ Bogus$Head | NumCards$ 1"),
			want: []string{"Bogus$Head"},
		},
		{
			// the head refuses (ActivationZone$ Nowhere, a zone kgen has no
			// name for) before its chain; the
			// body's own Defined$ blocker is the probe's to find
			name: "activated",
			src:  head("A:AB$ Draw | Cost$ 1 T | ActivationZone$ Nowhere | SorcerySpeed$ True | Defined$ Bogus$Head | NumCards$ 1 | SpellDescription$ Probe."),
			want: []string{"Bogus$Head"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := abLower(t, tc.src, true)
			if len(c.Refusals) == 0 {
				t.Fatal("precondition: the fixture lowers; its head must refuse")
			}
			rows := abBodyRows(c)
			all := strings.Join(rows, "\n")
			for _, bad := range []string{"cost shape", "outside a triggered ability", "api:Mana", "ActivationLimit", "SorcerySpeed", "Cost"} {
				if strings.Contains(all, bad) {
					t.Errorf("a probed body reads the activation head (%q):\n%s", bad, all)
				}
			}
			for _, w := range tc.want {
				if !strings.Contains(all, w) {
					t.Errorf("the probed body lacks its own blocker %q:\nbody rows:\n%s\nall:\n%s", w, all, abReasons(c))
				}
			}
		})
	}
}
