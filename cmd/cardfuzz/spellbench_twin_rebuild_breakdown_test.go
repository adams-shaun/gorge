package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/server/spellbench/neutral/twin"
)

// TestCardfuzzTwinRebuildBreakdown: a twin game result with one injected
// rebuild failure folds into the summary's rebuild classes, and the printed
// breakdown names the decision kind, the neutral world's id-free reason and
// the source card, with an example seed/decision. Before the breakdown
// existed the summary printed only the opaque total and the failure was
// invisible.
func TestCardfuzzTwinRebuildBreakdown(t *testing.T) {
	const (
		kind   = "priority"
		reason = "no native option for candidate kind activate_ability"
		card   = "Staff of Domination"
		seed   = uint64(7)
		index  = 42
	)
	sum := newSBTwinSummary()
	tw := twin.New()
	tw.Total[twin.Opaque] = 1 // the same decision the budget counts
	sum.add(seed, sbTwinGame{
		tw: tw,
		rebuilds: []twin.RebuildFailure{{
			Kind: kind, Index: index, Seat: "p1", Native: 3, Card: card, Reason: reason,
		}},
	})
	// Preconditions: the injected failure is where the breakdown looks and
	// the opaque total the rebuild budget reads.
	if got := sum.Rebuilds[rebuildClass(twin.RebuildFailure{Kind: kind, Reason: reason, Card: card})]; got == nil || got.N != 1 {
		t.Fatalf("injected rebuild failure did not fold into its class: %+v", sum.Rebuilds)
	}
	if got := sum.Total[twin.Opaque]; got != 1 {
		t.Fatalf("Total[Opaque] = %d, want 1", got)
	}
	var b bytes.Buffer
	sum.print(&b, sbOptions{mode: "twin", policy: "bot", autopay: true, seed: seed, games: 1, minAgree: 99.5})
	out := b.String()
	if !strings.Contains(out, "rebuild_class") {
		t.Fatalf("summary has no per-class rebuild breakdown:\n%s", out)
	}
	for _, want := range []string{kind, reason, card, "seed 7 decision 42"} {
		if !strings.Contains(out, want) {
			t.Fatalf("breakdown does not name %q:\n%s", want, out)
		}
	}
}
