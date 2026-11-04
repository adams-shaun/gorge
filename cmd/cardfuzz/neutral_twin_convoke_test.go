package main

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestTwinConvokeAnnouncementsAgree plays a Convoke X spell's deck with both agents
// behind one seat at a table that pays by hand, where a cast can need several
// creatures: the native bot and the seat rebuilding the ask from a plain observation
// tap the same ones.
func TestTwinConvokeAnnouncementsAgree(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	var list []string
	for _, n := range []struct {
		name  string
		count int
	}{{"Plains", 12}, {"Mountain", 12}, {"Grizzly Bears", 20}, {"Wildfire Awakener", 16}} {
		list = append(list, slices.Repeat([]string{n.name}, n.count)...)
	}
	decks := []genDeck{{Colour: "R", Cards: list}, {Colour: "R", Cards: list}}
	g := playSBTwin(reg, decks, 1, sbOptions{
		mode: "twin", policy: "bot", autopay: false,
		maxSteps: 20000, maxTurns: 100, maxIntents: 20000, maxObjects: 20000,
	})
	if g.err != "" {
		t.Fatalf("game failed: %s", g.err)
	}
	if g.tw.Total["select_object"] == 0 {
		t.Fatal("no selection was compared; the test proves nothing")
	}
	for _, f := range g.fails {
		if f.SB != nil && f.SB.Kind == "select_object" {
			t.Errorf("the agents disagree on an announcement: %s", f.Diag)
		}
	}
}
