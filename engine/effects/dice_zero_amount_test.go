package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// clownCarRoll resolves Clown Car's real TrigRollDice with the given X and a
// scripted Rand of a single even die (draw 1 → 2), returning the rolled dice,
// the car's +1/+1 counters and the host.
func clownCarRoll(t *testing.T, x int32) ([]int32, int32, *scriptedRollHost, *state.Game) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	cc, ok := reg.Lookup("Clown Car")
	if !ok {
		t.Fatal("corpus fixture: Clown Car missing")
	}
	g := state.NewGame([]string{"a", "b"})
	g.Tokens = reg.Tokens
	h := &scriptedRollHost{seq: []int{1, 1, 1}}
	h.g = g
	src := g.AddObject(cc, 0)
	src.Zone = state.ZBattlefield
	face := cc.Faces[0]
	sa := cards.ResolveSVar(face.SVars, "TrigRollDice")
	if sa == nil {
		t.Fatal("corpus fixture: Clown Car has no TrigRollDice SVar")
	}
	Resolve(h, &Ctx{Controller: 0, Source: src.ID, SVars: face.SVars, X: x}, sa)
	return rollResults(t, h), src.Counter("P1P1"), h, g
}

// TestRollDiceExplicitZeroAmountRollsNothing: Clown Car's copy was never cast,
// so X = 0 (CR 107.3m) and "roll X six-sided dice" rolls none (CR 706.2):
// no die Note, no counter, no Clown Robot token. The X = 2 control proves the
// same ability does roll when X is positive, so the zero case is not vacuous.
func TestRollDiceExplicitZeroAmountRollsNothing(t *testing.T) {
	rolls2, counters2, _, _ := clownCarRoll(t, 2)
	if len(rolls2) != 2 || counters2 != 2 {
		t.Fatalf("control: X=2 rolled %v with %d counters, want two even dice and 2 counters", rolls2, counters2)
	}
	rolls, counters, h, _ := clownCarRoll(t, 0)
	if len(rolls) != 0 {
		t.Fatalf("X=0 rolled %v, want no dice", rolls)
	}
	if counters != 0 {
		t.Fatalf("X=0 car has %d +1/+1 counters, want 0", counters)
	}
	if h.asked != nil {
		t.Fatalf("X=0 posed decision %v, want none", h.asked.Kind)
	}
	for _, e := range h.log {
		// PutCounter itself emits a no-op Amount 0 counter event; anything
		// that actually changes state is a failure.
		if e.Kind != events.Note && e.Amount != 0 {
			t.Fatalf("X=0 emitted %+v, want only Notes (no tokens, counters or roll)", e)
		}
	}
}
