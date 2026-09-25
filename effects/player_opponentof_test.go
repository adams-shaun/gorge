package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
	"github.com/adams-shaun/gorge/state"
)

func TestPlayerOpponentOfRememberedRequiresResolvedPlayer(t *testing.T) {
	g := state.NewGame(names(4))
	remembered := state.Target{Player: 1, IsPlayer: true}
	if remembered.Player == 2 {
		t.Fatal("test requires distinct remembered and candidate players")
	}
	ctx := PlayerSpecCtx{OpponentOf: []state.Target{remembered}}
	if !MatchesPlayerSpecCtx(g, "Player.OpponentOf Remembered", 2, 0, ctx) {
		t.Fatal("player 2 should be an opponent of remembered player 1")
	}
	if MatchesPlayerSpecCtx(g, "Player.OpponentOf Remembered", 1, 0, ctx) {
		t.Fatal("remembered player must not be their own opponent")
	}
	for name, invalid := range map[string]PlayerSpecCtx{
		"missing":       {},
		"object target": {OpponentOf: []state.Target{{Obj: 42}}},
		"out of range":  {OpponentOf: []state.Target{{Player: 9, IsPlayer: true}}},
		"unsupported":   {OpponentOf: []state.Target{{Player: 1, IsPlayer: true}}},
	} {
		spec := "Player.OpponentOf Remembered"
		if name == "unsupported" {
			spec = "Player.OpponentOf TriggeredPlayer"
		}
		if MatchesPlayerSpecCtx(g, spec, 2, 0, invalid) {
			t.Errorf("%s referent unexpectedly matched", name)
		}
	}
}

func TestChoosePlayerBendOrBreakOpponentOfRemembered(t *testing.T) {
	_, sa := corpusSA(t, "Bend or Break", "DBChoosePlayer")
	if sa.API != "ChoosePlayer" || sa.Params["Choices"] != "Player.OpponentOf Remembered" {
		t.Fatalf("Bend or Break ChoosePlayer fixture changed: %+v", sa)
	}
	remembered := state.PlayerID(1)
	candidate := state.PlayerID(2)
	if remembered == candidate {
		t.Fatal("test requires distinct remembered and selected players")
	}
	h := &askHost{fakeHost: *newHost(t, 4)}
	c := &Ctx{Controller: remembered, Remembered: []state.Target{{Player: remembered, IsPlayer: true}}}
	effChoosePlayer(h, c, sa)
	d := h.asked
	if d == nil || d.Kind != decision.KChoose || d.Player != remembered {
		t.Fatalf("Bend or Break did not ask remembered player 1: %+v", d)
	}
	want := []state.PlayerID{2, 3, 0}
	if len(d.Options) != len(want) {
		t.Fatalf("Bend or Break options = %+v, want opponents %v", d.Options, want)
	}
	for i, p := range want {
		if d.Options[i].Player != p || p == remembered {
			t.Fatalf("option %d = %+v, want opponent %d distinct from remembered player %d", i, d.Options[i], p, remembered)
		}
	}
	c.Choice = []state.Target{{Player: candidate, IsPlayer: true}}
	c.ChoiceDone = true
	effChoosePlayer(h, c, sa)
	if len(c.Chosen) != 1 || !c.Chosen[0].IsPlayer || c.Chosen[0].Player != candidate {
		t.Fatalf("selected pool result = %+v, want opponent player %d", c.Chosen, candidate)
	}
}
