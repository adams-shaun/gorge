package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

func TestTargetedPlayerCommanderCastCountUsesSelectedPlayer(t *testing.T) {
	g, ids := board(t)
	h := &fakeHost{g: g, commanderCasts: map[state.PlayerID]int32{0: 3, 1: 7}}
	obj := state.Target{Obj: ids["myBear"]}
	seat1 := state.Target{Player: 1, IsPlayer: true}
	seat0 := state.Target{Player: 0, IsPlayer: true}

	if h.CommanderCastsFromCommandZone(0) == h.CommanderCastsFromCommandZone(1) {
		t.Fatal("test precondition: commander-cast counts must differ")
	}
	ctx := &Ctx{Source: ids["myBear"], Controller: 0,
		Targets: []state.Target{obj, seat1}}
	if got := EvalCount(h, ctx, "TargetedPlayer$TotalCommanderCastFromCommandZone"); got != 7 {
		t.Fatalf("targeted commander-cast count = %d, want 7", got)
	}
	ctx.PickedTargets = []state.Target{seat0}
	if got := EvalCount(h, ctx, "ThisTargetedPlayer$TotalCommanderCastFromCommandZone"); got != 3 {
		t.Fatalf("picked targeted commander-cast count = %d, want 3", got)
	}
}
