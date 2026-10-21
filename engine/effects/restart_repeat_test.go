package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// repeatRestartHost is a fakeHost that accepts a CR 727 restart, so
// effRestartGame sets Ctx.Restarted instead of taking the bare-host draw
// fallback.
type repeatRestartHost struct {
	*fakeHost
	requests int
}

func (h *repeatRestartHost) RequestRestart(c *Ctx, keep []state.ObjID, rest *cards.SA) {
	h.requests++
}

// TestRepeatEachStopsAtRestart: RepeatEach resolves each iteration on a Ctx
// COPY, so a RestartGame body's Restarted flag must be carried back to the
// outer Ctx; otherwise the loop keeps resolving the remaining iterations in
// the old game (CR 727.4). Two subjects, so the pre-fix loop asks for two
// restarts.
func TestRepeatEachStopsAtRestart(t *testing.T) {
	h := &repeatRestartHost{fakeHost: newHost(t, 2)}
	if got := len(h.g.AliveFrom(0)); got != 2 {
		t.Fatalf("fixture precondition: %d alive seats, want 2", got)
	}
	root := &cards.SA{API: "RepeatEach", Params: map[string]string{
		"RepeatSubAbility": "Body",
		"RepeatPlayers":    "Player",
	}}
	c := &Ctx{Controller: 0, SVars: map[string]string{"Body": "DB$ RestartGame"}}
	effRepeatEach(h, c, root)
	if h.requests != 1 {
		t.Fatalf("RequestRestart calls = %d, want 1 (the loop must stop at the restart)", h.requests)
	}
	if !c.Restarted {
		t.Fatal("outer Ctx.Restarted = false; the resolving caller will not perform the restart")
	}
}
