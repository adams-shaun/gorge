package kgen

import (
	"strings"
	"testing"
)

// TestReplDrawEmptyGateLowers: the "while your library has no cards in it"
// Draw replacements (Ormos, Archive Keeper and Living Conundrum) lower with a
// library-size-zero condition and no Draw-replacement refusal. Ormos's
// counter-on-self body is a DrawProgram; Living Conundrum's bodyless Prevent$
// is the skip program.
func TestReplDrawEmptyGateLowers(t *testing.T) {
	const gate = "repl DrawProgram F=nil cond=Value(op3,LibrarySizeOf(You),0)"
	for _, tc := range []struct {
		name string
		body string // a body instruction the dump must carry, if any
	}{
		{"Ormos, Archive Keeper", "PutCounter"},
		{"Living Conundrum", ""},
	} {
		c := lowerNamedCard(t, tc.name)
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "Draw replacement") {
				t.Errorf("%s: Draw replacement still refused: %s", tc.name, r.Reason)
			}
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), gate) {
			t.Errorf("%s: lowered IR lacks the empty-library gate %q:\n%s", tc.name, gate, sb.String())
		}
		if tc.body != "" && !strings.Contains(sb.String(), tc.body) {
			t.Errorf("%s: lowered IR lacks the replacement body %q:\n%s", tc.name, tc.body, sb.String())
		}
	}
}
