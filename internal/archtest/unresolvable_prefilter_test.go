package archtest

import "testing"

// TestUnresolvablePrefilterNeverHidesAMatch proves the prefilter is an exact
// superset of unresolvableTag: every blob the regexp matches must also pass
// mayHaveUnresolvableTag, so a literal dropped from the prefilter shows up
// here rather than hiding a reference in the tracked tree.
func TestUnresolvablePrefilterNeverHidesAMatch(t *testing.T) {
	// The unresolvable-reference positives from TestPublicBoundaryDetection,
	// plus one blob per alternative of unresolvableTag. Some positives are
	// section citations rather than tag matches; findUnresolvable must still
	// report those, and their gate is the "§" filter, not this prefilter.
	for _, c := range []struct {
		path string
		blob string
	}{
		{"dirac/x.go", "// kr-loop fixed this\n"},
		{"draft/x.go", "// DESIGN §3.1\n"},
		{"dirac/x.md", "on wt/branch\n"},
		{"draft/x.md", "by design (operator, 2026-10-30)\n"},
		{"draft/x.md", "an Operator Ruling says\n"},
		{"draft/x.md", "an operator decision says\n"},
		{"engine/x.go", "// kr-loop, DESIGN §3.1, wt/branch, (operator) ruling\n"},
		{"draft/x.go", "// the rule E3-5 added\n"},
		{"draft/x.go", "// playing is E5-c2's seam\n"},
		{"draft/x.go", "// hub from E5d-4 (E0)\n"},
		{"dirac/x.md", "at E1 only\n"},
		{"draft/x.go", "// the spec §4.6 builder\n"},
		{"draft/x.go", "// plan §2\n"},
		{"draft/x.go", "// §3 step 4: over\n"},
		{"dirac/x.go", "// Ruling P2 holds\n"},
		{"draft/x.go", "// the brief's threshold\n"},
		{"cmd/labdemo/x.go", "// labdemo (E5-c4)\n"},
		{"cmd/draftsim/x.go", "// the spec §4.8 tool\n"},
		{"examples/extagent/x.py", "# E5d-6\n"},
		{"engine/x.go", "// E3-5, spec §4.6, Ruling P2, the brief\n"},
		{"draft/x.go", "x (Operator) y"},
		{"draft/x.go", "(task a"},
		{"draft/x.go", "E12"},
		{"draft/x.go", "wt/b"},
		{"draft/x.go", "Ruling P2"},
		{"draft/x.go", "brief's"},
		{"draft/x.go", "DESIGN §1"},
		{"draft/x.go", "kr-a"},
	} {
		b := []byte(c.blob)
		if unresolvableTag.Find(b) != nil {
			if !mayHaveUnresolvableTag(b) {
				t.Errorf("mayHaveUnresolvableTag(%.60q) = false, but unresolvableTag matches", c.blob)
			}
		} else if !sectionCite.Match(b) {
			// A section-cite positive: its "§" filter is exact, so the tag
			// prefilter is not involved.
			t.Fatalf("precondition: %.60q matches neither unresolvableTag nor sectionCite", c.blob)
		}
		if findUnresolvable(c.path, b) == nil {
			t.Fatalf("precondition: findUnresolvable(%q) does not match %.60q", c.path, c.blob)
		}
	}

	// A blob with none of the markers is skipped, and is genuinely clean.
	plain := []byte("package x\n// plain text 0x1f\n")
	if m := unresolvableTag.Find(plain); m != nil {
		t.Fatalf("precondition: fixture is not plain: %q matches", m)
	}
	if findUnresolvable("draft/x.go", plain) != nil {
		t.Fatalf("precondition: fixture is not clean: %q", findUnresolvable("draft/x.go", plain))
	}
	if mayHaveUnresolvableTag(plain) {
		t.Errorf("mayHaveUnresolvableTag(%q) = true, want false", plain)
	}
}
