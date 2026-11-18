package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDungeonsCompletedHeadReadsCompletedDungeons pins the head Ravenloft
// Adventurer's CheckSVar$ X gate reads ("if you've completed a dungeon"):
// DungeonsCompleted$Amount answers the resolving controller's
// Player.CompletedDungeons, a controller out of range reads 0, and any other
// suffix is NOT resolvable — the gate's caller fails closed on that verdict
// instead of enforcing a fake 0.
func TestDungeonsCompletedHeadReadsCompletedDungeons(t *testing.T) {
	h := newHost(t, 2)
	g := h.g
	// Precondition: the compared values differ (2 vs the zero field).
	g.Players[0].CompletedDungeons = 2
	c := &Ctx{Controller: 0}
	if got, ok := EvalCountOK(h, c, "DungeonsCompleted$Amount"); !ok || got != 2 {
		t.Fatalf("DungeonsCompleted$Amount = (%d,%v), want (2,true)", got, ok)
	}
	if got, ok := EvalCountOK(h, c, "DungeonsCompleted$Rooms"); ok {
		t.Fatalf("DungeonsCompleted$Rooms = (%d,%v), want not resolvable", got, ok)
	}
	out := &Ctx{Controller: 5}
	if got, ok := EvalCountOK(h, out, "DungeonsCompleted$Amount"); !ok || got != 0 {
		t.Fatalf("out-of-range controller = (%d,%v), want (0,true)", got, ok)
	}
}

// TestDefendingPlayerValidExileCountsOwnHitCounters pins the widened
// TriggeredDefendingPlayer Valid zone family (Ravenloft Adventurer's SVar:Y,
// "the defending player loses 1 life for each card they own in exile with a
// hit counter on it"): the count is cards in the DEFENDING player's exile
// zone that the referenced player (the filter's You) owns with a HIT counter
// on them, so a hit-counter card the opponent owns in that zone does not
// count and a counterless own card does not either.
func TestDefendingPlayerValidExileCountsOwnHitCounters(t *testing.T) {
	h := newHost(t, 2)
	g := h.g
	mine := "Name:My Card\nTypes:Creature\nOracle:x\n"
	theirs := "Name:Their Card\nTypes:Creature\nOracle:x\n"
	// Seat 1 is the defending player. Its exile holds three creatures: one
	// it owns with a hit counter, one it owns without, one owned by seat 0
	// with two hit counters.
	hit := putTestCard(t, g, 1, 1, state.ZExile, mine)
	g.Obj(hit).Counters = []state.Counter{{Kind: "HIT", N: 1}}
	putTestCard(t, g, 1, 1, state.ZExile, mine)
	theirsID := putTestCard(t, g, 0, 1, state.ZExile, theirs)
	g.Obj(theirsID).Counters = []state.Counter{{Kind: "HIT", N: 2}}

	c := &Ctx{Controller: 0}
	c.DefendingPlayer = state.Target{Player: 1, IsPlayer: true}
	if got, ok := EvalCountOK(h, c, "TriggeredDefendingPlayer$ValidExile Card.YouOwn+counters_GE1_HIT"); !ok || got != 1 {
		t.Fatalf("TriggeredDefendingPlayer$ValidExile Card.YouOwn+counters_GE1_HIT = (%d,%v), want (1,true)", got, ok)
	}
}
