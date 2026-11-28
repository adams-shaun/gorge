package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestSearchMirrorCandidateShape pins how the search mirror compares two
// candidate lists: a declaration's closing Done is not an action (the kernel
// lists it, gorge does not), a real difference is classed by which side holds
// the extra candidate, and only sets posed against single picks are a
// multi-select.
func TestSearchMirrorCandidateShape(t *testing.T) {
	a := Pack(ActAttack, 7, RefNone, 0)
	b := Pack(ActAttack, 33, RefNone, 0)
	done := Pack(ActDone, RefNone, RefNone, 0)
	if a == b || a.Kind() == ActDone || done.Kind() != ActDone {
		t.Fatalf("fixture actions are not distinct: %v %v %v", a, b, done)
	}

	gorge := [][]Canon{{a, b}, {}, {a}}
	kernel := [][]Canon{{b, a, done}, {done}, {a, done}}
	if got := candShape(gorge, kernel); got != "" {
		t.Errorf("declarations equal but for Done: shape %q, want none", got)
	}
	if got := showCanon(kernel[0]); len(got) != 2 {
		t.Errorf("showCanon kept the Done: %v", got)
	}
	for _, tc := range []struct {
		name   string
		g, k   [][]Canon
		expect string
	}{
		{"gorge holds one more", [][]Canon{{a}, {b}}, [][]Canon{{a}}, "gorge-more"},
		{"kernel holds one more", [][]Canon{{a}}, [][]Canon{{a}, {b}}, "kernel-more"},
		{"each holds its own", [][]Canon{{a}}, [][]Canon{{b}}, "differ"},
		{"multiplicity differs", [][]Canon{{a}, {a}, {b}}, [][]Canon{{a}, {b}, {b}}, "differ"},
	} {
		if got := candShape(tc.g, tc.k); got != tc.expect {
			t.Errorf("%s: shape %q, want %q", tc.name, got, tc.expect)
		}
	}

	two := &decision.Decision{Kind: decision.KTarget, Max: 2}
	sets, singles := [][]Canon{{a, b}, {a}}, [][]Canon{{a}, {b}}
	if !multiSelect(two, sets, singles) {
		t.Error("sets against single picks at a two-target decision: want multi-select")
	}
	if multiSelect(two, singles, singles) {
		t.Error("single options on both sides: not a multi-select difference")
	}
	if multiSelect(&decision.Decision{Kind: decision.KTarget, Max: 1}, sets, singles) {
		t.Error("a one-target decision poses no multi-select")
	}
	if multiSelect(&decision.Decision{Kind: decision.KAttackers, Max: 2}, sets, singles) {
		t.Error("a declaration is whole on both sides: not a multi-select difference")
	}

	cut := &SearchMirrorMismatch{Kind: "target", What: MirrorCandidates, Shape: "limit-cut"}
	more := &SearchMirrorMismatch{Kind: "priority", What: MirrorCandidates, Shape: "kernel-more"}
	if cut.Class() != "candidates/limit-cut" || more.Class() != "candidates/priority/kernel-more" {
		t.Errorf("classes %q and %q", cut.Class(), more.Class())
	}
}
