package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// Forge's TwoPilesEffect clears the host's remembered list once both pile
// bodies ran, unless KeepRemembered$ or RememberChosen$ is present (Hostile
// Negotiations' YouChoose keeps it for TheyChoose). These tests pin the clear,
// its two opt-outs and that an empty clear is not an event.

// runKeepRemembered resolves a Fact-or-Fiction-shaped TwoPiles with the given
// extra params over a source holding two remembered cards. It returns the
// host, the source id and the Ctx the resolution ran with.
func runKeepRemembered(t *testing.T, extra string) (*fakeHost, state.ObjID, *Ctx) {
	t.Helper()
	h, ids := twopilesBoard(t)
	src := ids[5]
	h.g.Obj(src).Remembered = twopilesRemembered(ids[:2])
	if len(h.g.Obj(src).Remembered) != 2 {
		t.Fatal("precondition: the source must hold two remembered cards")
	}
	ctx := &Ctx{Controller: 0, Source: src, Remembered: twopilesRemembered(ids[:5]), SVars: twopilesSVars()}
	Resolve(h, ctx, sa(t, "DB$ TwoPiles | Defined$ You | DefinedCards$ Remembered | Separator$ Opponent | ChosenPile$ DBHand | UnchosenPile$ DBGrave"+extra))
	if h.g.Obj(ids[0]).Zone != state.ZHand {
		t.Fatalf("precondition: the pile bodies must have run (pile A zone = %d)", h.g.Obj(ids[0]).Zone)
	}
	return h, src, ctx
}

func countClears(h *fakeHost) int {
	n := 0
	for _, e := range h.log {
		if e.Kind == events.Choose && e.Counter == "clear-remembered" {
			n++
		}
	}
	return n
}

func TestTwoPilesDefaultClearsSourceRemembered(t *testing.T) {
	t.Parallel()
	h, src, ctx := runKeepRemembered(t, "")
	if got := len(h.g.Obj(src).Remembered); got != 0 {
		t.Fatalf("source Remembered has %d entries after a default TwoPiles, want 0", got)
	}
	if len(ctx.Remembered) != 0 {
		t.Fatalf("Ctx.Remembered = %+v, want empty", ctx.Remembered)
	}
	if n := countClears(h); n != 1 {
		t.Fatalf("clear-remembered events = %d, want exactly 1", n)
	}
}

func TestTwoPilesKeepRememberedKeeps(t *testing.T) {
	t.Parallel()
	h, src, ctx := runKeepRemembered(t, " | KeepRemembered$ True")
	if got := len(h.g.Obj(src).Remembered); got != 2 {
		t.Fatalf("source Remembered has %d entries with KeepRemembered$, want 2", got)
	}
	if len(ctx.Remembered) != 5 {
		t.Fatalf("Ctx.Remembered has %d entries with KeepRemembered$, want 5", len(ctx.Remembered))
	}
	if n := countClears(h); n != 0 {
		t.Fatalf("clear-remembered events = %d, want 0", n)
	}
}

func TestTwoPilesRememberChosenKeeps(t *testing.T) {
	t.Parallel()
	h, src, ctx := runKeepRemembered(t, " | RememberChosen$ True")
	if got := len(h.g.Obj(src).Remembered); got != 2 {
		t.Fatalf("source Remembered has %d entries with RememberChosen$, want 2", got)
	}
	// The no-host stand-in takes pile A (the first card); it is appended to
	// the five-card set the resolution already held.
	if len(ctx.Remembered) != 6 || ctx.Remembered[5].Obj != ctx.Remembered[0].Obj {
		t.Fatalf("Ctx.Remembered = %+v, want the set plus the chosen pile appended", ctx.Remembered)
	}
	if n := countClears(h); n != 0 {
		t.Fatalf("clear-remembered events = %d, want 0", n)
	}
}
