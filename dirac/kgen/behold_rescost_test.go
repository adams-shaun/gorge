package kgen

import (
	"strings"
	"testing"
)

// TestBeholdResolutionCost: Behold<1/Type> as a resolution-time cost lowers
// to a pay instruction that moves nothing: an UnlessCost$ read with
// UnlessSwitched (Elven Passage) and a printed trigger's own cost (Sarkhan,
// Dragon Ascendant).
func TestBeholdResolutionCost(t *testing.T) {
	want := map[string]string{
		"Elven Passage":             "UnlessSwitched obj=You A=1 CS=Behold",
		"Sarkhan, Dragon Ascendant": "IfPaid obj=You A=1 CS=Behold",
	}
	var names []string
	for n := range want {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		frag, ok := want[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), frag) {
			t.Errorf("%s: lowered IR lacks %q:\n%s", c.Name, frag, sb.String())
		}
	}
	if seen != len(want) {
		t.Errorf("lowered %d of %d carriers", seen, len(want))
	}
}
