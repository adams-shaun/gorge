package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
)

func countClearChosenColor(h *fakeHost) int {
	n := 0
	for _, e := range h.log {
		if e.Kind == events.Choose && e.Counter == "clear-chosen-color" {
			n++
		}
	}
	return n
}

// TestCleanupClearChosenColor: ClearChosenColor$ True empties the source's
// recorded colour with exactly one clear event; a plain Cleanup twin leaves
// it; an already-empty source emits no clear (only the Note).
func TestCleanupClearChosenColor(t *testing.T) {
	seed := func(h *fakeHost) *Ctx {
		src := h.g.AddObject(mkCard(t, "Name:Src\nTypes:Instant\nOracle:x\n"), 0)
		h.Emit(events.Event{Kind: events.Choose, Obj: src.ID, Counter: "color", Text: "U"})
		if got := h.g.Obj(src.ID).ChosenColor; got != "U" {
			t.Fatalf("precondition: seeded ChosenColor = %q, want U", got)
		}
		return &Ctx{Controller: 0, Source: src.ID}
	}

	hClear := newHost(t, 2)
	cClear := seed(hClear)
	Resolve(hClear, cClear, sa(t, "DB$ Cleanup | ClearChosenColor$ True"))

	hTwin := newHost(t, 2)
	cTwin := seed(hTwin)
	Resolve(hTwin, cTwin, sa(t, "DB$ Cleanup"))

	if got := hClear.g.Obj(cClear.Source).ChosenColor; got != "" {
		t.Fatalf("clearing cleanup left ChosenColor = %q, want empty", got)
	}
	if got := hTwin.g.Obj(cTwin.Source).ChosenColor; got != "U" {
		t.Fatalf("twin cleanup left ChosenColor = %q, want U", got)
	}
	if n := countClearChosenColor(hClear); n != 1 {
		t.Fatalf("clear-chosen-color events = %d, want 1: %+v", n, hClear.log)
	}
	if n := countClearChosenColor(hTwin); n != 0 {
		t.Fatalf("twin emitted %d clear-chosen-color events", n)
	}

	hEmpty := newHost(t, 2)
	src := hEmpty.g.AddObject(mkCard(t, "Name:Src\nTypes:Instant\nOracle:x\n"), 0)
	if hEmpty.g.Obj(src.ID).ChosenColor != "" {
		t.Fatal("precondition: fresh source already has a colour")
	}
	Resolve(hEmpty, &Ctx{Controller: 0, Source: src.ID}, sa(t, "DB$ Cleanup | ClearChosenColor$ True"))
	if n := countClearChosenColor(hEmpty); n != 0 {
		t.Fatalf("empty source emitted %d clear-chosen-color events", n)
	}
	if len(hEmpty.log) != 1 || hEmpty.log[0].Kind != events.Note {
		t.Fatalf("empty-source log = %+v, want only the Note", hEmpty.log)
	}
}
