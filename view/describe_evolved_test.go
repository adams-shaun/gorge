// This file is package view (internal), matching describe_test.go: it reuses
// that file's describeFixture, which builds the game through identity_test.go's
// package-private boltSrc.
package view

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/events"
)

// TestDescribeEvolvedNamesTheEvolvingPermanent pins the transcript line for the
// events.Evolved marker (CR 702.99b, task trig:Evolved). The marker emits no
// state of its own -- the +1/+1 counter has its own CounterChange line -- so
// Describe must name the evolving permanent rather than fall through to the
// "unknown event" fallback that TestDescribeCoversEveryKind guards against.
func TestDescribeEvolvedNamesTheEvolvingPermanent(t *testing.T) {
	g, bear, _ := describeFixture(t)

	// Precondition: the object exists and carries a printed face, so obj()
	// renders its name. A zero id or a faceless object would make the
	// assertion below pass for the wrong reason.
	o := g.Obj(bear)
	if o == nil || o.Face() == nil || o.Face().Name == "" {
		t.Fatalf("precondition: Bear object is not a nameable permanent: %+v", o)
	}

	line := Describe(g, events.Event{Kind: events.Evolved, Obj: bear, Player: 0})
	if line == "unknown event" {
		t.Fatalf("events.Evolved fell through to the fallback, not its own case: %q", line)
	}
	if !strings.Contains(line, "Bear") {
		t.Fatalf("Evolved line %q does not name the evolving permanent Bear", line)
	}
	if !strings.Contains(line, "evolves") {
		t.Fatalf("Evolved line %q does not record the evolve action", line)
	}
}
