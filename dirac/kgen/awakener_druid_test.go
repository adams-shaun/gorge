package kgen

import (
	"strings"
	"testing"
)

// TestAsLongAsInPlayAnimateWithPTLowers pins the lowering of the three corpus
// cards whose Animate carries Duration$ AsLongAsInPlay and a literal P/T:
// Awakener Druid, Skilled Animator and Unctus's Retrofitter. AsLongAsInPlay is
// the animating permanent's lifetime (gorge animateHostScoped, CR 611.2b), so
// each is a host-bound override that keeps its P/T in the table. Before this,
// the duration was refused by name and none reached a pool.
func TestAsLongAsInPlayAnimateWithPTLowers(t *testing.T) {
	for _, c := range []struct {
		name, pt string
	}{
		{"Awakener Druid", "pt=4/5"},
		{"Skilled Animator", "pt=5/5"},
		{"Unctus's Retrofitter", "pt=4/4"},
	} {
		t.Run(c.name, func(t *testing.T) {
			cards := lowerNamed(t, []string{c.name})
			if len(cards) != 1 || cards[0].Name != c.name {
				t.Fatalf("lowered %d cards, want %s", len(cards), c.name)
			}
			var refusals []string
			for _, r := range cards[0].Refusals {
				refusals = append(refusals, r.Reason)
			}
			if len(refusals) != 0 {
				t.Fatalf("%s refused: %s", c.name, strings.Join(refusals, "; "))
			}
			var dump strings.Builder
			dumpLIR(&dump, cards[0])
			got := dump.String()
			// The P/T is constant table data, and the duration must be the
			// host-bound record, not an end-of-turn or permanent animation.
			for _, want := range []string{
				"Override obj=Target0 CS=WhileHost",
				"override{" + c.pt,
			} {
				if !strings.Contains(got, want) {
					t.Errorf("%s: lowered IR lacks %q:\n%s", c.name, want, got)
				}
			}
		})
	}
}
