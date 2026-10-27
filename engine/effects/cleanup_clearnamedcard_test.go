package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

func countClearNamedCard(h *fakeHost) int {
	n := 0
	for _, e := range h.log {
		if e.Kind == events.Choose && e.Counter == "clear-named-card" {
			n++
		}
	}
	return n
}

// TestCleanupClearNamedCard: ClearNamedCard$ True empties the source's
// recorded card name with exactly one clear event; a plain Cleanup twin
// leaves it; an already-empty source emits no clear (only the Note). The
// name is seeded through events.Choose so the precondition is the same
// state path a NameCard resolution writes.
func TestCleanupClearNamedCard(t *testing.T) {
	seed := func(h *fakeHost) *Ctx {
		src := h.g.AddObject(mkCard(t, "Name:Src\nTypes:Instant\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.Choose, Obj: src.ID, Counter: "name", Text: "Lightning Bolt"})
		if got := h.g.Obj(src.ID).ChosenName; got != "Lightning Bolt" {
			t.Fatalf("precondition: seeded ChosenName = %q, want Lightning Bolt", got)
		}
		return &Ctx{Controller: 0, Source: src.ID}
	}

	hClear := newHost(t, 2)
	cClear := seed(hClear)
	Resolve(hClear, cClear, sa(t, "DB$ Cleanup | ClearNamedCard$ True"))

	hTwin := newHost(t, 2)
	cTwin := seed(hTwin)
	Resolve(hTwin, cTwin, sa(t, "DB$ Cleanup"))

	if got := hClear.g.Obj(cClear.Source).ChosenName; got != "" {
		t.Fatalf("clearing cleanup left ChosenName = %q, want empty", got)
	}
	if got := hTwin.g.Obj(cTwin.Source).ChosenName; got != "Lightning Bolt" {
		t.Fatalf("twin cleanup left ChosenName = %q, want Lightning Bolt", got)
	}
	if n := countClearNamedCard(hClear); n != 1 {
		t.Fatalf("clear-named-card events = %d, want 1: %+v", n, hClear.log)
	}
	if n := countClearNamedCard(hTwin); n != 0 {
		t.Fatalf("twin emitted %d clear-named-card events", n)
	}

	hEmpty := newHost(t, 2)
	src := hEmpty.g.AddObject(mkCard(t, "Name:Src\nTypes:Instant\nOracle:x\n"), 0)
	if hEmpty.g.Obj(src.ID).ChosenName != "" {
		t.Fatal("precondition: fresh source already has a name")
	}
	Resolve(hEmpty, &Ctx{Controller: 0, Source: src.ID}, sa(t, "DB$ Cleanup | ClearNamedCard$ True"))
	if n := countClearNamedCard(hEmpty); n != 0 {
		t.Fatalf("empty source emitted %d clear-named-card events", n)
	}
	if len(hEmpty.log) != 1 || hEmpty.log[0].Kind != events.Note {
		t.Fatalf("empty-source log = %+v, want only the Note", hEmpty.log)
	}
}
