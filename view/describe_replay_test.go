// This file is package view_test, not view, for the same reason
// visibility_test.go is (see that file's own doc comment): it needs
// seat.Bot to drive a real game, and seat imports view (bot.go takes a
// view.View) — an internal (package view) test file that also imported
// seat would be a genuine import cycle for Go's test tooling (view[.test]
// -> seat -> view). This test touches only exported symbols (Describe,
// Project), so package view_test costs it nothing.
package view_test

import (
	"context"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/internal/testutil"
	"github.com/adams-shaun/gorge/rules"
	"github.com/adams-shaun/gorge/seat"
	"github.com/adams-shaun/gorge/view"
)

// TestDescribeIsIdenticalAcrossTwoRunsOfTheSameMatch plays the same seeded
// match twice and describes every event as it goes: Describe must be a
// pure function of (g, ev), so the two transcripts have to match line for
// line. This is the strongest determinism check in the suite — over 500
// real events, not a handful of hand-built cases.
func TestDescribeIsIdenticalAcrossTwoRunsOfTheSameMatch(t *testing.T) {
	run := func() []string {
		names, decks := testutil.SampleDecks(t, 4)
		e := rules.New(rules.Config{Seed: 21, Names: names, Decks: decks})
		e.Advance()
		b := seat.NewBot(21)
		var lines []string
		describeFrom := func(from int) {
			for _, ev := range e.L.Events[from:] {
				lines = append(lines, view.Describe(e.G, ev))
			}
		}
		describeFrom(0)
		for i := 0; i < 300 && !e.G.Over && e.Pending() != nil; i++ {
			d := e.Pending()
			from := len(e.L.Events)
			in, _ := b.Decide(context.Background(), view.Project(e.G, e, d.Player, d), *d)
			if err := e.Submit(in); err != nil {
				t.Fatal(err)
			}
			describeFrom(from)
		}
		return lines
	}
	a, b := run(), run()
	if strings.Join(a, "\n") != strings.Join(b, "\n") {
		t.Fatal("Describe produced different lines for two runs of the same match")
	}
	if len(a) < 500 {
		t.Fatalf("fixture produced only %d lines", len(a))
	}
}
