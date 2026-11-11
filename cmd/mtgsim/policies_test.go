package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/bots"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRunPlaysEveryKnownPolicy is the regression test for mtgsim building
// named-policy seats without the card registry: every policy bots.Names()
// lists must actually play a game when selected with -a. sb-tactical reads
// printed card facts from Deps.Cards and refuses a nil registry, so a seat
// built without it fails before the first decision.
func TestRunPlaysEveryKnownPolicy(t *testing.T) {
	names := bots.Names()
	if len(names) < 6 {
		t.Fatalf("bots.Names() has %d policies (%v), want at least 6 linked by bots/all", len(names), names)
	}
	found := false
	for _, name := range names {
		if name == "sb-tactical" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("sb-tactical is not among bots.Names() (%v); bots/all no longer links it", names)
	}

	dir := testutil.CorpusDirOrSkip(t)
	for _, name := range names {
		name := name
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := runAgents("", 2, 0, 1, true, false, dir, name, "bot", nil, &buf); err != nil {
				t.Fatalf("runAgents with -a %s: %v\n%s", name, err, buf.String())
			}
			out := buf.String()
			if !strings.Contains(out, "replay OK") {
				t.Fatalf("policy %s did not replay:\n%s", name, out)
			}
			if !strings.Contains(out, "winner=") && !strings.Contains(out, "draw") {
				t.Fatalf("policy %s produced no result:\n%s", name, out)
			}
			if strings.Contains(out, "FAILED") {
				t.Fatalf("policy %s failed:\n%s", name, out)
			}
		})
	}
}
