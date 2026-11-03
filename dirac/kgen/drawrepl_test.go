package kgen

import (
	"strings"
	"testing"
)

// TestDrawReplacementLowers: the controller's "draw N cards instead"
// replacement lowers to ReplDrawMore (a granted max-speed one too), and the
// optional choice-body shape stays refused.
func TestDrawReplacementLowers(t *testing.T) {
	carriers := []string{"Mr. Spock, Science Officer", "Alhammarret's Archive", "Bard, King of Dale", "Vnwxt, Verbose Host"}
	for _, c := range lowerNamed(t, carriers) {
		for _, r := range c.Refusals {
			t.Errorf("%s: refused: %s", c.Name, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), "repl DrawMore") {
			t.Errorf("%s: lowered IR lacks the draw replacement:\n%s", c.Name, sb.String())
		}
	}
	for _, c := range lowerNamed(t, []string{"Pavel Chekov, First Posting"}) {
		found := false
		for _, r := range c.Refusals {
			found = found || strings.Contains(r.Reason, "Draw replacement with an optional or choice body")
		}
		if !found {
			t.Errorf("%s: no choice-body refusal (%v)", c.Name, c.Refusals)
		}
	}
}
