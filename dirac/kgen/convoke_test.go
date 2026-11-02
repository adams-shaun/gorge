package kgen

import (
	"strings"
	"testing"
)

// TestConvokeXRefused: a printed Convoke on a cost with {X} is refused
// (xOpen absorption and xAsk pricing are not ported).
func TestConvokeXRefused(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Transcendent Message"}) {
		if c.Name != "Transcendent Message" {
			continue
		}
		var reasons []string
		for _, r := range c.Refusals {
			reasons = append(reasons, r.Node+": "+r.Reason)
		}
		got := strings.Join(reasons, "; ")
		if !strings.Contains(got, "Convoke with {X}") {
			t.Fatalf("refusals %q, want Convoke with {X}", got)
		}
		return
	}
	t.Fatal("Transcendent Message not lowered")
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
