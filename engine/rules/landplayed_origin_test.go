package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestLandPlayedOriginScopesThePlay pins Mode$ LandPlayed's Origin$ and its
// non-hand plays (CR 305.1: playing a land is a special action, also under a
// permission to play it from another zone). Shadow of the Goblin's "Whenever
// you play a land ... from anywhere other than your hand" (Origin$
// Ante,Command,Exile,Graveyard,Library) must fire on a land played from the
// graveyard under Crucible of Worlds and must NOT fire on a hand land drop;
// the matcher used to admit only hand-to-battlefield moves and ignore Origin$,
// so both answers were inverted.
func TestLandPlayedOriginScopesThePlay(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name, scenario string
		want           bool
	}{
		{"graveyard play fires", `{"name":"x","cr":["305.1"],"why":"x","setup":{"p0":{"battlefield":["Shadow of the Goblin","Crucible of Worlds"],"graveyard":["Forest"],"hand":["Wastes"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"play","seat":0,"card":"p0:Forest"},{"op":"pass_to","seat":0,"decision":"priority"}]}`, true},
		{"hand play does not fire", `{"name":"x","cr":["305.1"],"why":"x","setup":{"p0":{"battlefield":["Shadow of the Goblin"],"hand":["Wastes","Forest"]},"p1":{"battlefield":["Grizzly Bears"]}},"steps":[{"op":"pass","seat":0},{"op":"pass","seat":1},{"op":"pass_to","seat":0,"decision":"priority"},{"op":"play","seat":0,"card":"p0:Forest"},{"op":"pass_to","seat":0,"decision":"priority"}]}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			res, err := RunOracleScenarioJSON(reg, []byte(tc.scenario))
			if err != nil || len(res.Fails) != 0 {
				t.Fatalf("scenario: err=%v fails=%v", err, res.Fails)
			}
			got := false
			for _, e := range res.Snapshots[len(res.Snapshots)-1].Stack {
				got = got || (e.Kind == "ability" && e.Trigger == "1")
			}
			if got != tc.want {
				t.Fatalf("Undying Vengeance on the stack = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLandPutFromHandIsNotPlayed: an effect that puts a land from hand onto
// the battlefield is not the land-play special action (CR 305.4), so a
// "whenever you play a land" trigger (Horn of Greed) stays silent. It mirrors
// the graveyard arm below: only the engine's land-play fact admits the move.
func TestLandPutFromHandIsNotPlayed(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	src := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Horn of Greed"))
	o := e.G.AddObject(mustCorpusCard(t, reg, "Forest"), 0)
	o.Zone = state.ZHand
	e.G.SetZone(state.ZHand, 0, append(e.G.Zone(state.ZHand, 0), o.ID))
	// Precondition: the land is in the zone the matcher reads. Nothing arms
	// the land-play fact for this hand->battlefield move, so it is a put.
	if got := e.G.Obj(o.ID).Zone; got != state.ZHand {
		t.Fatalf("Forest zone before the put = %v, want hand", got)
	}
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZHand, To: state.ZBattlefield})
	if got := e.G.Obj(o.ID).Zone; got != state.ZBattlefield {
		t.Fatalf("Forest zone after the put = %v, want battlefield (the move must not be a no-op)", got)
	}
	if n := observedTriggerCount(e, src); n != 0 {
		t.Fatalf("a land put onto the battlefield from hand queued %d land-play triggers, want 0", n)
	}
}

// TestLandPutFromGraveyardIsNotPlayed: a land merely put onto the battlefield
// from a graveyard (not the land-play special action) is not "played" (CR
// 305.1, 305.4), so Shadow of the Goblin's non-hand land-play trigger stays
// silent.
func TestLandPutFromGraveyardIsNotPlayed(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := layerEngine(t)
	src := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Shadow of the Goblin"))
	o := e.G.AddObject(mustCorpusCard(t, reg, "Forest"), 0)
	o.Zone = state.ZGraveyard
	e.G.SetZone(state.ZGraveyard, 0, append(e.G.Zone(state.ZGraveyard, 0), o.ID))
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZGraveyard, To: state.ZBattlefield})
	if n := observedTriggerCount(e, src); n != 0 {
		t.Fatalf("a land put onto the battlefield from the graveyard queued %d land-play triggers, want 0", n)
	}
}
