package kgen

import (
	"strings"
	"testing"
)

// TestGhostVacuumSweep pins a ChangeZoneAll sweep
// onto the battlefield reads WithCountersType$ and StaticEffect$ as per
// entrant riders (kop.MvEntryRiders), and the two neighbouring carriers stay
// refused for their own reasons.
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
			if !strings.Contains(got, "AddColor") {
				t.Errorf("Grimoire of the Dead: want a refusal naming AddColor, got %q", got)
			}
		case "Shilgengar, Sire of Famine":
			seen++
			if !strings.Contains(got, "AB$ Token") {
				t.Errorf("Shilgengar: want the A[0] Token refusal, got %q", got)
			}
		}
	}
	if seen != 3 {
		t.Errorf("lowered %d of the 3 named cards", seen)
	}
}
