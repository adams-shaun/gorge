package kgen

import (
	"strings"
	"testing"
)

// TestGhostVacuumSweep pins a ChangeZoneAll sweep
// onto the battlefield reads WithCountersType$ and StaticEffect$ as per
// entrant riders (kop.MvEntryRiders). Ghost Vacuum's neighbour Grimoire of
// the Dead's AddColor$ rider lowers now (its STUDY counter refuses at emit,
// past this stage); Shilgengar's ChangeZoneAll arm
// lowers, and its Angel sacrifice gate lowers with it.
func TestGhostVacuumSweep(t *testing.T) {
	cs := lowerNamed(t, []string{"Ghost Vacuum", "Grimoire of the Dead", "Shilgengar, Sire of Famine"})
	seen := 0
	for _, c := range cs {
		var why []string
		for _, r := range c.Refusals {
			why = append(why, r.Reason)
		}
		got := strings.Join(why, "; ")
		switch c.Name {
		case "Ghost Vacuum":
			seen++
			if got != "" {
				t.Errorf("Ghost Vacuum refused: %s", got)
			}
			var sb strings.Builder
			dumpLIR(&sb, c)
			lir := sb.String()
			for _, want := range []string{"ChangeZoneAll", "counters=Flying", "static="} {
				if !strings.Contains(lir, want) {
					t.Errorf("Ghost Vacuum LIR lacks %q:\n%s", want, lir)
				}
			}
		case "Grimoire of the Dead":
			seen++
			if strings.Contains(got, "AddColor") {
				t.Errorf("Grimoire of the Dead: AddColor$ refused: %q", got)
			}
		case "Shilgengar, Sire of Famine":
			seen++
			// The Angel gate is the same sacrifice-time creature-type read as
			// Gowron's, so its Token arm now lowers and the card builds whole.
			if got != "" {
				t.Errorf("Shilgengar refused: %s", got)
			}
			var sb strings.Builder
			dumpLIR(&sb, c)
			lir := sb.String()
			for _, want := range []string{"ChangeZoneAll", "RefCount({types=0x0 sub=Angel},Sacrificed)"} {
				if !strings.Contains(lir, want) {
					t.Errorf("Shilgengar LIR lacks %q:\n%s", want, lir)
				}
			}
		}
	}
	if seen != 3 {
		t.Errorf("lowered %d of the 3 named cards", seen)
	}
}
