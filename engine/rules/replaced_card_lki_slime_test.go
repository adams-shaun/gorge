package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestRavenousSlimeExilesOpponentsDyingCreatureWithItsPower pins the real
// replacement path (replCtx -> runReplaceWith), not a hand-built Ctx: an
// opponent's creature with a +1/+1 counter dies, Ravenous Slime's Moved
// replacement exiles it instead of the graveyard, and its PutCounter body
// puts counters equal to that creature's power as it died
// (X:ReplacedCardLKI$CardPower). The counter-heavy power (3, not the printed
// 2) must be read while the creature is still on the battlefield -- by the
// time the body's ChangeZone to exile has run, its counters are cleared -- so
// replCtx must capture the derived P/T at replacement match.
func TestRavenousSlimeExilesOpponentsDyingCreatureWithItsPower(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	slime := mustCorpusCard(t, reg, "Ravenous Slime")
	bear := mustCorpusCard(t, reg, "Grizzly Bears")
	e, cfg := censusEngine(t, 7331, []*cards.Card{slime}, []*cards.Card{bear})
	slimeID := moveOwnerCard(t, e, 0, slime, state.ZBattlefield)
	bearID := moveOwnerCard(t, e, 1, bear, state.ZBattlefield)
	e.G.Obj(bearID).Counters = append(e.G.Obj(bearID).Counters, state.Counter{Kind: "P1P1", N: 1})
	if p := e.Power(bearID); p != 3 {
		t.Fatalf("precondition: the bear's power is %d, want 3 (printed 2 plus one +1/+1 counter)", p)
	}

	// Lethal damage through the ordinary SBA path (the afterlife test's shape).
	e.emit(events.Event{Kind: events.Damage, Obj: bearID, Amount: 99})
	e.checkStateBased()
	e.priorityRound()
	passUntilStackEmpty(t, e, 20)

	if o := e.G.Obj(bearID); o.Zone != state.ZExile {
		t.Fatalf("the opponent's dying bear is in %s, want exile (Ravenous Slime's replacement)", o.Zone)
	}
	got := int32(0)
	for _, k := range e.G.Obj(slimeID).Counters {
		if k.Kind == "P1P1" {
			got += k.N
		}
	}
	if got != 3 {
		t.Fatalf("Ravenous Slime has %d +1/+1 counters, want 3 (the bear's power as it died)", got)
	}
	replayCheck(t, e, cfg)
}
