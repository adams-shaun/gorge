package oracle

import (
	"path/filepath"
	"strings"
	"testing"
)

// TestCapacityBoundsReprosPlayPast: the two games that once stopped at the
// dynamic-mana capacity (a fifth one-colour source; an amount past 40) run
// 200 decisions beyond their script with no kernel stall. A stall is not a
// divergence, so the lockstep table alone would pass them either way.
func TestCapacityBoundsReprosPlayPast(t *testing.T) {
	reg := synthReg(t)
	for _, name := range []string{"fra_fifth_one_colour_source", "tmt_dynamic_mana_past_40"} {
		t.Run(name, func(t *testing.T) {
			r, err := ReadRepro(filepath.Join("testdata", "repro", name+".repro.json"))
			if err != nil {
				t.Fatal(err)
			}
			res := replayStdReproResult(t, reg, "custom", name, false)
			if res.Div != nil {
				t.Fatalf("divergence at %d: %s %s %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("kernel stall %q after %d of %d scripted decisions", res.Stall, res.Decisions, len(r.Actions))
			}
			if res.Decisions <= len(r.Actions) {
				t.Fatalf("%d decisions, want more than the script's %d", res.Decisions, len(r.Actions))
			}
		})
	}
}
