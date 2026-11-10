package kgen

import (
	"strings"
	"testing"
)

// delayedRememberObjectsCarriers: delayed registrations whose RememberObjects$
// names one object the lowering can resolve. Each lowers with no refusal and
// its lowered IR registers the object the form names.
var delayedRememberObjectsCarriers = map[string]struct{ form, lir string }{
	"U.S.S. Farragut, Bold Explorer": {
		"a triggered ability's Remembered set that its own hand-to-battlefield choice of one card wrote",
		"DelayedTrigger obj=Remembered C=1 CS=NextEndStep"},
}

func TestDelayedRememberObjects(t *testing.T) {
	var names []string
	for n := range delayedRememberObjectsCarriers {
		names = append(names, n)
	}
	seen := 0
	for _, c := range lowerNamed(t, names) {
		want, ok := delayedRememberObjectsCarriers[c.Name]
		if !ok {
			continue
		}
		seen++
		for _, r := range c.Refusals {
			t.Errorf("%s (%s): refused: %s", c.Name, want.form, r.Reason)
		}
		var sb strings.Builder
		dumpLIR(&sb, c)
		if !strings.Contains(sb.String(), want.lir) {
			t.Errorf("%s (%s): lowered IR lacks %q:\n%s", c.Name, want.form, want.lir, sb.String())
		}
	}
	if seen != len(delayedRememberObjectsCarriers) {
		t.Errorf("lowered %d of %d carriers", seen, len(delayedRememberObjectsCarriers))
	}
}

// TestDelayedRememberObjectsRefusesSets: Dragonhawk's chain exiles the top X
// cards and remembers all of them, so its delayed record would hold several
// objects (it carries one): both its triggers refuse by name.
func TestDelayedRememberObjectsRefusesSets(t *testing.T) {
	cs := lowerNamed(t, []string{"Dragonhawk, Fate's Tempest"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cs))
	}
	want := `delayed trigger RememberObjects "Remembered" in a triggered ability: a set of several objects`
	n := 0
	for _, r := range cs[0].Refusals {
		if strings.Contains(r.Reason, want) {
			n++
		}
	}
	if n != 2 {
		t.Errorf("%d refusals name %q, want 2 (enters and attacks): %v", n, want, cs[0].Refusals)
	}
}

// TestDelayedRememberObjectsRefusesSacrificed: gorge still finds a sacrificed
// card after it moved from the graveyard to exile (its incarnation advances
// only across the battlefield boundary) where the kernel's program does not,
// so The Guardian of Forever refuses by name.
func TestDelayedRememberObjectsRefusesSacrificed(t *testing.T) {
	cs := lowerNamed(t, []string{"The Guardian of Forever"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want 1", len(cs))
	}
	want := `delayed trigger RememberObjects "SacrificedCards": gorge still finds the card`
	for _, r := range cs[0].Refusals {
		if strings.Contains(r.Reason, want) {
			return
		}
	}
	t.Errorf("refusals %v, want %q", cs[0].Refusals, want)
}
