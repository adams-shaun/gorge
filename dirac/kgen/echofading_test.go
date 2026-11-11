package kgen

import (
	"strings"
	"testing"
)

// TestEchoFadingLowering pins the lowering of the echo and fading keywords
// (CR 702.30a, 702.32a; gorge kwEcho / kwFading): an echo trigger is gated by
// CondEchoOwed and is the source-gone skip (ExtCumulative B=1), the
// pay-or-decline ask over the printed mana cost (generic and coloured pips)
// and the sacrifice of the source; fading is the fade counters on entry and a
// sacrifice chained ahead of the removal.
func TestEchoFadingLowering(t *testing.T) {
	want := map[string][]string{
		"Cradle Guard":     {"cond=EchoOwed()", "Cumulative B=1 C=2", "UnlessPaid obj=You A=1 B=8192 CS=Mana skip=1", "Sacrifice obj=You A=1 F={types=0x0 Self}"},
		"Mogg War Marshal": {"cond=EchoOwed()", "UnlessPaid obj=You A=1 B=512 CS=Mana skip=1"},
		"Avalanche Riders": {"cond=EchoOwed()", "UnlessPaid obj=You A=3 B=512 CS=Mana skip=1"},
		"Blastoderm": {"EnterWithCounters obj=Replaced A=3 CS=FADE", "Sacrifice obj=You A=1 F={types=0x0 Self}",
			"RemoveCounter obj=Self A=1 CS=FADE"},
	}
	names := make([]string, 0, len(want))
	for n := range want {
		names = append(names, n)
	}
	for _, c := range lowerNamed(t, names) {
		if len(c.Refusals) != 0 {
			t.Fatalf("%s refused: %v", c.Name, c.Refusals)
		}
		var dump strings.Builder
		dumpLIR(&dump, c)
		for _, w := range want[c.Name] {
			if !strings.Contains(dump.String(), w) {
				t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, w, dump.String())
			}
		}
	}
}

// TestEchoRefusesOtherCosts: an echo cost that is not plain mana stays refused
// by name, never read as a mana one: {X} (Volcano Hellion), a discard (Deepcavern Imp) and a sacrifice of lands (Skizzik Surger).
func TestEchoRefusesOtherCosts(t *testing.T) {
	refused := map[string]bool{}
	for _, name := range []string{"Volcano Hellion", "Pouncing Jaguar", "Skizzik Surger", "Deepcavern Imp"} {
		for _, c := range lowerNamed(t, []string{name}) {
			for _, r := range c.Refusals {
				if strings.Contains(r.Reason, "echo cost") {
					refused[name] = true
				}
			}
		}
	}
	if !refused["Volcano Hellion"] {
		t.Errorf("Volcano Hellion's {X} echo should refuse by name; refusals: %v", refused)
	}
	if !refused["Skizzik Surger"] || !refused["Deepcavern Imp"] {
		t.Errorf("the discard and sacrifice echo costs should refuse by name; refusals: %v", refused)
	}
	if refused["Pouncing Jaguar"] {
		t.Errorf("Pouncing Jaguar's plain {G} echo should lower")
	}
}
