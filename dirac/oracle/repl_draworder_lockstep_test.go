package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// TestReplDrawOrderLockstep: two Draw replacement programs matching one draw
// (Notion Thief and Hullbreacher on seat 0, both naming the opponent's
// draws) are gorge's CR 616.1 order ask for the drawing seat; the kernel
// poses the same ask (repl_draworder.go) instead of stalling. Seat 1's
// Divination draws are the competition: the draw step's first card is exempt
// from both lines, so only the spell's two cards meet them.
func TestReplDrawOrderLockstep(t *testing.T) {
	t.Parallel()
	decks := [2][]cardset.Entry{
		{{Name: "Notion Thief", Count: 4}, {Name: "Hullbreacher", Count: 4}, {Name: "Grizzly Bears", Count: 8},
			{Name: "Hill Giant", Count: 4}, {Name: "Swamp", Count: 10}, {Name: "Island", Count: 10}},
		{{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 4},
			{Name: "Divination", Count: 4}, {Name: "Island", Count: 16}, {Name: "Forest", Count: 8}},
	}
	before := dirac.DrawOrderStats.Asked.Load()
	lockstepSet(t, "draw-repl", decks, 400, "KDRAWORDER_SEEDS")
	if got := dirac.DrawOrderStats.Asked.Load() - before; got == 0 {
		t.Fatal("no game posed the draw order ask: the pool never put two draw programs on one draw")
	}
}
