package kgen

import (
	"strings"
	"testing"
)

// TestConvokeXLowers: a printed Convoke on a cost with {X} lowers (the
// kernel's announcement and X ask price the picks, kw_convoke.go), with no
// refusal left on the card.
func TestConvokeXLowers(t *testing.T) {
	n := 0
	for _, c := range lowerNamed(t, []string{"Transcendent Message", "Chord of Calling"}) {
		if c.Name != "Transcendent Message" && c.Name != "Chord of Calling" {
			continue
		}
		n++
		if len(c.Refusals) != 0 {
			t.Fatalf("%s refused: %+v", c.Name, c.Refusals)
		}
		if !c.Convoke || !c.Cost.X {
			t.Fatalf("%s: convoke=%v X=%v, want a convoke {X} cost", c.Name, c.Convoke, c.Cost.X)
		}
	}
	if n != 2 {
		t.Fatalf("lowered %d of the two cards", n)
	}
}

// TestImproviseXRefused: a printed Improvise on a cost with {X} is refused
// (xAsk pricing is not ported).
func TestImproviseXRefused(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Whir of Invention"}) {
		if c.Name != "Whir of Invention" {
			continue
		}
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		got := strings.Join(reasons, "; ")
		if !strings.Contains(got, "Improvise with {X}") {
			t.Fatalf("refusals %q, want Improvise with {X}", got)
		}
		return
	}
	t.Fatal("Whir of Invention not lowered")
}
