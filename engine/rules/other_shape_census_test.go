package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEveryOtherShapeCardIsPlayable is the class census. The names under
// playable must carry NO unsupported primitive; the gated names must carry
// exactly their named gate and nothing else, so a family widened without the
// census moving (or a gate silently dropped) is caught here.
func TestEveryOtherShapeCardIsPlayable(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	sup := effects.Supported()
	playable := []string{
		"Anti-Magic Aura", "Consecrate Land", // stat:CantAttach Aura shapes
		"Bend or Break", "Death or Glory", // api:TwoPiles
		"Ore-Rich Stalactite", // api:Craft (ExileFromGrave material head)
	}
	for _, name := range playable {
		c := corpusCardNamed(t, name)
		if got := reg.Unsupported(c, sup); len(got) != 0 {
			t.Errorf("%s: unsupported = %v, want none", name, got)
		}
	}
	gated := map[string]string{
		"Eye of Ojer Taq": "api:Craft.OtherShape",            // withSharedCardType: a set relation the per-object material matcher cannot read
		"Raging River":    "api:TwoPiles.OtherShape",         // LeftRightPile$: left/right blocking labels, a new mechanism
		"Guardian Beast":  "stat:CantGainControl",            // its CantAttach Aura half is implemented (other_shape_gates_test.go); the control-change static is a different family
		"Imperiosaur":     "stat:ManaRestriction.OtherShape", // spend-only-from-basic-lands: per-source pool provenance state.Player does not track
		"Myr Superion":    "stat:ManaRestriction.OtherShape", // spend-only-from-creatures: the same provenance gap
		"Deification":     "repl:RemoveCounter.OtherShape",   // damage arm rewrites removal ("all but one") inside the damage path
		"Time Vault":      "repl:BeginTurn.OtherShape",       // optional skip of your own turn, not an extra turn
	}
	for name, want := range gated {
		c := corpusCardNamed(t, name)
		got := reg.Unsupported(c, sup)
		if len(got) != 1 || got[0] != want {
			t.Errorf("%s: unsupported = %v, want exactly [%s]", name, got, want)
		}
	}
}
