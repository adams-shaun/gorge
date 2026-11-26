package kgen

import (
	"strings"
	"testing"
)

// TestDragonhawkLowering pins Dragonhawk, Fate's Tempest's lowering: both of
// its dig triggers (enters and attacks) delay an end-step trigger that
// remembers the whole dug set (RememberObjects$ Remembered over a Dig's
// RememberChanged$ cards) and deals 2 damage per still-exiled card to each
// opponent. The kernel's DpMany program carries the set (dirac/
// trig_delayedmany.go) and each body is one DamageAll whose amount is the
// still-exiled remembered count doubled (DelayedExileCount*2), followed by
// the registration's Cleanup ClearRemembered$.
func TestDragonhawkLowering(t *testing.T) {
	cs := lowerNamed(t, []string{"Dragonhawk, Fate's Tempest"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("refused: %s", r.Reason)
	}
	var sb strings.Builder
	dumpLIR(&sb, c)
	ir := sb.String()
	for _, want := range []string{
		// each trigger registers the dug set as one DpMany program (B=3);
		// the bodies and the clear are the shared shapes, one per trigger
		"DelayedTrigger obj=Remembered B=3 C=1 CS=YourNextEndStep",
		"DelayedTrigger obj=Remembered B=3 C=3 CS=YourNextEndStep",
	} {
		if !strings.Contains(ir, want) {
			t.Errorf("lowered IR lacks %q:\n%s", want, ir)
		}
	}
	for _, want := range []string{
		"DamageAll A=DelayedExileCount*2 F={players=2}",
		"ClearRemembered",
	} {
		if n := strings.Count(ir, want); n != 2 {
			t.Errorf("%d occurrences of %q, want 2:\n%s", n, want, ir)
		}
	}
}
