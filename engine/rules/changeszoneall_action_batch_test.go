package rules

// Mode$ ChangesZoneAll ("whenever ONE OR MORE cards are put into ...") fires
// once for a whole mill, discard or surveil, not once per moved card (CR
// 603.2c; Forge's MillEffect/DiscardEffect/SurveilEffect each fire
// triggerChangesZoneAll once from one CardZoneTable). The watcher is an
// authored fixture, seated eventlessly (so no replayCheck: a log-only replay
// cannot rebuild a hand-placed permanent); Hedge Shredder is the real compiled corpus card.

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// zoneWatcherSrc is a permanent that gains its controller 2 life whenever one
// or more of their cards are put into a graveyard from origin.
func zoneWatcherSrc(origin string) string {
	return "Name:Zone Watcher\nManaCost:0\nTypes:Artifact\n" +
		"T:Mode$ ChangesZoneAll | ValidCards$ Card.YouOwn | Origin$ " + origin +
		" | Destination$ Graveyard | TriggerZones$ Battlefield | Execute$ TrigGain | TriggerDescription$ x\n" +
		"SVar:TrigGain:DB$ GainLife | LifeAmount$ 2\nOracle:x\n"
}

// countPushes counts the TriggerPush events naming obj from event index
// start on, and fails when the watcher never queued at all.
func countPushes(e *Engine, obj state.ObjID, start int) int {
	n := 0
	for _, ev := range e.L.Events[start:] {
		if ev.Kind == events.TriggerPush && ev.Obj == obj {
			n++
		}
	}
	return n
}

// zoneWatcherGame seats the watcher on seat 0's battlefield over Mountain
// decks and returns the engine, its config and the watcher's id.
func zoneWatcherGame(t *testing.T, seed uint64, origin string) (*Engine, Config, state.ObjID) {
	t.Helper()
	e, cfg := tokenReplGame(t, seed)
	w := onBoardCard(t, e, 0, card(t, zoneWatcherSrc(origin)))
	if o := e.G.Obj(w); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("watcher %d is not on the battlefield (vacuous setup)", w)
	}
	return e, cfg, w
}

func TestChangesZoneAllMillFiresOnce(t *testing.T) {
	t.Parallel()
	e, _, w := zoneWatcherGame(t, 9205, "Any")
	if n := len(e.G.Zone(state.ZLibrary, 0)); n < 3 {
		t.Fatalf("library holds %d cards, need 3 to mill", n)
	}
	start, life := len(e.L.Events), e.G.Players[0].Life
	resolveMill(t, e, 0, 3)
	e.priorityRound()
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 3 {
		t.Fatalf("graveyard holds %d cards after milling 3, want 3 (precondition)", got)
	}
	drainMillTrigger(t, e, 40)
	if got := countPushes(e, w, start); got != 1 {
		t.Fatalf("watcher TriggerPush count = %d, want 1 (one mill is one zone-change event)", got)
	}
	if got := e.G.Players[0].Life - life; got != 2 {
		t.Fatalf("life gained = %d, want 2", got)
	}
}

func TestChangesZoneAllDiscardFiresOnce(t *testing.T) {
	t.Parallel()
	e, _, w := zoneWatcherGame(t, 9206, "Any")
	emptyHandToLibrary(t, e, 0)
	for i := 0; i < 3; i++ {
		seedHand(t, e, 0, nonlandDiscardCard)
	}
	if got := len(e.G.Zone(state.ZHand, 0)); got != 3 {
		t.Fatalf("hand holds %d cards, want 3 (precondition)", got)
	}
	start, life := len(e.L.Events), e.G.Players[0].Life
	resolveDiscardHand(t, e, 0)
	if got := len(e.G.Zone(state.ZGraveyard, 0)); got != 3 {
		t.Fatalf("graveyard holds %d cards after discarding 3, want 3 (precondition)", got)
	}
	drainMillTrigger(t, e, 40)
	if got := countPushes(e, w, start); got != 1 {
		t.Fatalf("watcher TriggerPush count = %d, want 1 (one discard is one zone-change event)", got)
	}
	if got := e.G.Players[0].Life - life; got != 2 {
		t.Fatalf("life gained = %d, want 2", got)
	}
}

func TestChangesZoneAllSurveilFiresOnce(t *testing.T) {
	t.Parallel()
	e, _, id := newFixtureDeck(t, 9207, surveil3Src)
	addMana(t, e, 0, "U")
	// Origin$ Library: the resolved surveil spell itself reaching the
	// graveyard from the stack is a separate event and must not count.
	w := onBoardCard(t, e, 0, card(t, zoneWatcherSrc("Library")))
	d := surveilDecision(t, e, id)
	if len(d.Options) != 3 {
		t.Fatalf("surveil window = %d cards, want 3 (precondition)", len(d.Options))
	}
	start, life := len(e.L.Events), e.G.Players[0].Life
	submitRest(t, e, nil, nil) // keep none: all three go to the graveyard
	for _, o := range d.Options {
		if got := e.G.Obj(o.Obj).Zone; got != state.ZGraveyard {
			t.Fatalf("surveilled card %d zone = %v, want graveyard (precondition)", o.Obj, got)
		}
	}
	passUntilStackEmpty(t, e, 20)
	if got := countPushes(e, w, start); got != 1 {
		t.Fatalf("watcher TriggerPush count = %d, want 1 (one surveil is one zone-change event)", got)
	}
	if got := e.G.Players[0].Life - life; got != 2 {
		t.Fatalf("life gained = %d, want 2", got)
	}
	if e.zoneBatchOpen || e.zoneBatchDepth != 0 {
		t.Fatalf("zone batch left open (depth %d)", e.zoneBatchDepth)
	}
}

func TestHedgeShredderReturnsBothLandsOnce(t *testing.T) {
	t.Parallel()
	e, sh, libIDs, graveIDs := hedgeShredderEngine(t, 9208, []string{"Forest", "Mountain"}, []string{"Island"})
	start := len(e.L.Events)
	effects.Resolve(e, &effects.Ctx{Controller: 0},
		&cards.SA{Kind: "DB", API: "Mill", Params: map[string]string{"NumCards": "2"}})
	e.priorityRound()
	for _, id := range libIDs {
		if got := zoneOfObj(e, id); got != state.ZGraveyard {
			t.Fatalf("milled land %d zone = %v, want graveyard (precondition)", id, got)
		}
	}
	hsDrain(t, e)
	if got := countPushes(e, sh, start); got != 1 {
		t.Fatalf("Hedge Shredder TriggerPush count = %d, want 1", got)
	}
	for i, id := range libIDs {
		if o := e.G.Obj(id); o.Zone != state.ZBattlefield || !o.Tapped {
			t.Fatalf("milled land #%d zone=%v tapped=%v, want tapped on the battlefield", i, o.Zone, o.Tapped)
		}
	}
	if got := zoneOfObj(e, graveIDs[0]); got != state.ZGraveyard {
		t.Fatalf("unrelated graveyard land zone = %v, want graveyard", got)
	}
}
