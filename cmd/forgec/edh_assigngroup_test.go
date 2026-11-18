package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestEDHAssignGroupSupported pins the friend-or-foe class: every card whose
// spell is SP$ AssignGroup must be playable under the coverage report's
// predicate (primitive census plus the ETB shape gate).
func TestEDHAssignGroupSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range []string{"Pir's Whim", "Khorvath's Fury", "Zndrsplt's Judgment", "Regna's Sanction", "Virtus's Maneuver"} {
		c, ok := r.Lookup(name)
		if !ok {
			t.Errorf("corpus missing %s", name)
			continue
		}
		if m := r.Unsupported(c, sup); len(m) != 0 {
			t.Errorf("%s unsupported: %v", name, m)
		}
	}
}
