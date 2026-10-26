package view

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestDescribeSecretChooseForOpponent pins the redacted Secret Choose line:
// an opponent sees that the chooser made a choice, never the chosen type. The
// chooser's own copy keeps the ordinary full line.
func TestDescribeSecretChooseForOpponent(t *testing.T) {
	g, bear, _ := describeFixture(t)
	ev := events.Event{Kind: events.Choose, Obj: bear, Counter: "type",
		Text: "Goblin", Player: 0, Secret: true}

	own := RedactEvent(g, ev, 0)
	if line := Describe(g, own); line != "Ann chooses the type Goblin for Bear #1" {
		t.Fatalf("chooser's own line = %q, want the full type line", line)
	}

	opp := RedactEvent(g, ev, 1)
	if opp.Obj != 0 || opp.Text != "" || opp.Counter != "" {
		t.Fatalf("precondition: opponent's Choose not redacted to shape: %+v", opp)
	}
	line := Describe(g, opp)
	if line != "Ann secretly makes a choice" {
		t.Fatalf("opponent's line = %q, want \"Ann secretly makes a choice\"", line)
	}
	if strings.Contains(line, "Goblin") {
		t.Fatalf("opponent's line leaked the chosen type: %q", line)
	}
}
