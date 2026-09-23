package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestMakeThemPayChoosesTheGreatestLifeOpponent(t *testing.T) {
	card, sa := corpusSA(t, "The Master, Gallifrey's End", "DBChoosePlayer")
	if sa.API != "ChoosePlayer" || sa.Params["Choices"] != "Player.Opponent+lifeEQX" {
		t.Fatalf("fixture SA = %+v", sa)
	}
	h := newHost(t, 4)
	source := h.g.AddObject(card, 0)
	h.Emit(events.Event{Kind: events.MoveZone, Obj: source.ID, From: state.ZLibrary, To: state.ZBattlefield})
	h.g.Players[1].Life, h.g.Players[2].Life, h.g.Players[3].Life = 20, 40, 30
	ctx := &Ctx{Source: source.ID, Controller: 0, SVars: source.Face().SVars}
	got, ok := EvalCountOK(h, ctx, source.Face().SVars["X"])
	if !ok || got != 40 {
		t.Fatalf("Make Them Pay X = %d, %v; want greatest opponent life 40, true", got, ok)
	}
	if !MatchesPlayerSpecWithSVars(h, ctx, "Player.Opponent+lifeEQX", 2, 0) ||
		MatchesPlayerSpecWithSVars(h, ctx, "Player.Opponent+lifeEQX", 1, 0) {
		t.Fatal("lifeEQX did not isolate the greatest-life opponent")
	}
	defined, ok := definedSpec(h, ctx, "Player.Opponent+lifeEQX")
	if !ok || len(defined) != 1 || defined[0].Player != 2 {
		t.Fatalf("defined Player bridge = %+v, %v; want only seat 2", defined, ok)
	}
	Resolve(h, ctx, sa)
	if len(ctx.Chosen) != 1 || !ctx.Chosen[0].IsPlayer || ctx.Chosen[0].Player != 2 {
		t.Fatalf("ChoosePlayer chose %+v; want greatest-life seat 2 (not first candidate seat 1)", ctx.Chosen)
	}
}

func TestLifeThresholdSVar(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	h := &fakeHost{g: g}
	g.Players[0].Life, g.Players[1].Life = 5, 12
	c := &Ctx{Controller: 0, SVars: map[string]string{"X": "8"}}
	if !MatchesPlayerSpecWithSVars(h, c, "Player.lifeGE8", 1, 0) {
		t.Fatal("literal lifeGE8 stopped matching")
	}
	if !MatchesPlayerSpecWithSVars(h, c, "Player.lifeGTX", 1, 0) ||
		MatchesPlayerSpecWithSVars(h, c, "Player.lifeGTX", 0, 0) {
		t.Fatal("lifeGTX failed to match exactly the seat above resolved threshold")
	}
	if MatchesPlayerSpecWithSVars(h, c, "Player.lifeEQUndefined", 1, 0) {
		t.Fatal("unresolvable threshold matched a player")
	}
}
