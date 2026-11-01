package main

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/draft/limited/setdata"
)

// TestDraftsimUnknownSetRefused pins the unknown-SET error (as opposed to
// the unknown-collation one): it names the set as unknown and does NOT
// trail an empty collations list.
func TestDraftsimUnknownSetRefused(t *testing.T) {
	// Precondition: ZZZ is not an embedded set, so Collations yields nil —
	// the distinction the error message below rests on.
	if setdata.Collations("ZZZ") != nil {
		t.Fatal("precondition: ZZZ unexpectedly carries collations")
	}
	o := options{
		set: "ZZZ", seed: 1, events: 1,
		players: "bot,bot,bot,bot", drafter: "rating-colour", builder: "best23", dir: ".",
	}
	_, err := run(o, &strings.Builder{})
	if err == nil {
		t.Fatal("run accepted an unknown set")
	}
	if !strings.Contains(err.Error(), `unknown set "ZZZ"`) {
		t.Errorf("unknown-set error %q does not name the set unknown", err)
	}
	if strings.Contains(err.Error(), "collations:") {
		t.Errorf("unknown-set error %q still trails a collations list", err)
	}
}
