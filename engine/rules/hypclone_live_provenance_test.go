package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

// TestHypCloneSiblingKeepsLiveParentPrefix: a spent clone of a live
// hypothetical clone c keeps its provenance in the pool (c's array is
// append-only history while c lives), the next clone of c takes that Spare
// and its log is exactly c's; once c is released no pooled Spare names c's
// array any more, so a later tenant of that array is copied in full
// (TestHypCloneOfACloneNeverKeepsAStalePrefix's hazard).
func TestHypCloneSiblingKeepsLiveParentPrefix(t *testing.T) {
	e, _ := fdnCensusEngine(t, "Grizzly Bears")
	root := e.hypClone(e)
	c := e.hypClone(root)
	c.emit(events.Event{Kind: events.Note, Text: "c-a"})
	c.emit(events.Event{Kind: events.Note, Text: "c-b"})
	arr := &c.L.Events[0]

	g1 := e.hypClone(c)
	g1.emit(events.Event{Kind: events.Note, Text: "g1-only"})
	e.hypRelease(g1)
	kept := false
	for _, sp := range e.hypPool().spares {
		if from, _ := sp.Provenance(); from == arr {
			kept = true
		}
	}
	if !kept {
		t.Fatal("the spent sibling's Spare lost its provenance while its parent is alive")
	}

	c.emit(events.Event{Kind: events.Note, Text: "c-c"})
	g2 := e.hypClone(c)
	if len(g2.L.Events) != len(c.L.Events) || g2.L.Head() != c.L.Head() {
		t.Fatalf("sibling log %d events head %s, parent %d head %s", len(g2.L.Events), g2.L.Head(), len(c.L.Events), c.L.Head())
	}
	for i := range c.L.Events {
		if g2.L.Events[i].Kind != c.L.Events[i].Kind || g2.L.Events[i].Text != c.L.Events[i].Text {
			t.Fatalf("sibling event %d = %s %q, parent has %s %q", i, g2.L.Events[i].Kind, g2.L.Events[i].Text, c.L.Events[i].Kind, c.L.Events[i].Text)
		}
	}
	e.hypRelease(g2)
	e.hypRelease(c)
	for _, sp := range e.hypPool().spares {
		if from, _ := sp.Provenance(); from == arr {
			t.Fatal("a pooled Spare still names a released clone's array")
		}
	}
	for _, lv := range e.hypPool().live {
		if lv.eng == c {
			t.Fatal("a released clone is still listed live")
		}
	}
	e.hypRelease(root)
	if n := len(e.hypPool().live); n != 0 {
		t.Fatalf("%d clones still listed live after every release", n)
	}
}
