package main

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// edhDayTimeChangesCards is the EDH census class "T:Mode$ DayTimeChanges":
// cards whose "whenever day becomes night or night becomes day" trigger must
// count supported (The Celestus, Component Collector, Vadrik, Astral
// Archmage). The matcher is trigmatch's dayTimeChangesMatches; this test pins
// that the coverage census registers trig:DayTimeChanges, so the three cards
// are counted playable.
var edhDayTimeChangesCards = []string{
	"The Celestus",
	"Component Collector",
	"Vadrik, Astral Archmage",
}

func TestEDHDayTimeChangesSupported(t *testing.T) {
	r := testutil.CorpusRegistry(t)
	r.ShapeGate = etbCopyShapeGate
	defer func() { r.ShapeGate = nil }()
	sup := effects.Supported()
	for _, name := range edhDayTimeChangesCards {
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
