package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// Bash to Bits' flashback ({4}{R}{R}) was offered by gorge as a cast from the
// graveyard in a priority decision where the kernel offered none: p1's board
// tapped for five mana with only one {R} (a Mountain), and the only second {R}
// sat behind Energy Refractor's own {2} activation cost (decision 393 of 393,
// tail-02-c000 pair 55, seed 648290).
func TestMBCBashToBitsReprosLockstep(t *testing.T) {
	t.Parallel()
	dirac.ZzFiltOn() // TEMP PROBE
	stdReproNamed(t, "flashback_cast_priced_over_ability_cost")
}

// TestZZBashPanicDetail TEMP PROBE: print the kernel panic's stack.
func TestZZBashPanicDetail(t *testing.T) {
	dirac.ZzFiltOn()
	r, err := ReadRepro("testdata/repro/flashback_cast_priced_over_ability_cost.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	res := replayDecodedScript(t, synthReg(t), globSet(t, r.Pool, r), r, &ScriptPolicy{Actions: r.Actions})
	if res.Div != nil {
		t.Fatalf("div at %d: %s", res.Div.At, res.Div.Detail)
	}
	t.Fatal("no divergence")
}
