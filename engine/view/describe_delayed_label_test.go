package view

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestDescribeDelayedRegisterHidesLabelTrailer pins that the "|LB=<description>"
// trailer a delayed-trigger registration carries (the label the rules side
// reads back) never reaches the event log: the line names the phase only.
func TestDescribeDelayedRegisterHidesLabelTrailer(t *testing.T) {
	g, _, _ := describeFixture(t)
	bare := Describe(g, events.Event{Kind: events.DelayedRegister, Text: "Upkeep"})
	if !strings.HasSuffix(bare, " for Upkeep") {
		t.Fatalf("precondition: bare registration = %q, want a line ending \" for Upkeep\"", bare)
	}
	for _, text := range []string{"Upkeep|LB=Draw a card.", "Upkeep|CR=1|LB=Draw a card."} {
		got := Describe(g, events.Event{Kind: events.DelayedRegister, Text: text})
		if got != bare {
			t.Errorf("Text %q described as %q, want %q", text, got, bare)
		}
	}
}
