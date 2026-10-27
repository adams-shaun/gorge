package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestRememberDrawnRecordsHostRemembered pins that RememberDrawn$ writes BOTH
// halves of the remembered state: the resolution's Ctx.Remembered (what a
// chained sub-ability reads this walk) and the source object's event-backed
// Remembered list (what a later Count$RememberedSize reads). Oblivious
// Bookworm's discard rider is the user: the draw remembered the card in the
// Ctx but the discard's Count$RememberedSize saw nothing, so its NumCards$
// resolved to zero and the discard did nothing.
func TestRememberDrawnRecordsHostRemembered(t *testing.T) {
	h := newHost(t, 2)
	card := mkCard(t, "Name:Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n")
	src := h.g.AddObject(card, 0)
	top := h.g.AddObject(card, 0)
	c := &Ctx{Source: src.ID, Controller: 0}

	// Precondition: both halves start empty and Count$RememberedSize reads the
	// host object's list (the source is where the rule looks), not the Ctx's.
	if len(c.Remembered) != 0 || len(h.g.Obj(src.ID).Remembered) != 0 {
		t.Fatalf("precondition: remembered not empty: ctx=%v host=%v",
			c.Remembered, h.g.Obj(src.ID).Remembered)
	}
	if n, ok := EvalCountOK(h, c, "Count$RememberedSize"); !ok || n != 0 {
		t.Fatalf("precondition: Count$RememberedSize = %d ok=%v, want 0 true", n, ok)
	}

	recordDrawn(h, c, 0, nil, []state.ObjID{top.ID}, false)

	if len(c.Remembered) != 1 || c.Remembered[0].Obj != top.ID {
		t.Fatalf("ctx Remembered = %v, want exactly [%d]", c.Remembered, top.ID)
	}
	host := h.g.Obj(src.ID).Remembered
	if len(host) != 1 || host[0].Obj != top.ID {
		t.Fatalf("host Remembered = %v, want exactly [%d]", host, top.ID)
	}
	if n, ok := EvalCountOK(h, c, "Count$RememberedSize"); !ok || n != 1 {
		t.Fatalf("Count$RememberedSize = %d ok=%v, want 1 true", n, ok)
	}
}
