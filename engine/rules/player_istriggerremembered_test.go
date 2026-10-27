package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestPlayerIsTriggerRememberedMatchesCapturedPlayer pins the PLAYER spelling
// of Forge's IsTriggerRemembered (PlayerProperty): a seat qualifies when it is
// a player entry of the delayed-trigger registration's capture, the set the
// rules-side phase gate binds as PlayerSpecCtx.DelayedRemembered. Before the
// arm existed, The Eternal Wanderer's `ValidPlayer$ Player.IsTriggerRemembered`
// matched nobody and the exiled card never returned.
func TestPlayerIsTriggerRememberedMatchesCapturedPlayer(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	src := onBoard(t, e, 0, "Name:Watcher\nTypes:Creature\nPT:2/2\nOracle:x\n")
	if o := e.G.Obj(src); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: source not on the battlefield: %+v", o)
	}
	bound := effects.PlayerSpecCtx{
		Source:            src,
		DelayedRemembered: []state.Target{{IsPlayer: true, Player: 1}},
	}
	if got := effects.MatchesPlayerSpecCtx(e.G, "Player.IsTriggerRemembered", 1, 0, bound); !got {
		t.Error("captured player 1 did not match Player.IsTriggerRemembered")
	}
	if effects.MatchesPlayerSpecCtx(e.G, "Player.IsTriggerRemembered", 0, 0, bound) {
		t.Error("uncaptured player 0 matched Player.IsTriggerRemembered")
	}
	// Object entries in the same capture never satisfy a PLAYER read.
	objectOnly := effects.PlayerSpecCtx{Source: src, DelayedRemembered: []state.Target{{Obj: src}}}
	if effects.MatchesPlayerSpecCtx(e.G, "Player.IsTriggerRemembered", 1, 0, objectOnly) {
		t.Error("object-only capture matched Player.IsTriggerRemembered")
	}
	// An empty/absent registration capture fails closed for every seat.
	for _, pc := range []effects.PlayerSpecCtx{{Source: src}, {}} {
		for _, p := range []state.PlayerID{0, 1} {
			if effects.MatchesPlayerSpecCtx(e.G, "Player.IsTriggerRemembered", p, 0, pc) {
				t.Errorf("empty capture matched Player.IsTriggerRemembered for seat %d", p)
			}
		}
	}
	// `!` negates the captured membership, and an absent capture must not
	// invert the fail-closed miss into admitting every seat.
	if !effects.MatchesPlayerSpecCtx(e.G, "Player.!IsTriggerRemembered", 0, 0, bound) {
		t.Error("Player.!IsTriggerRemembered did not admit the uncaptured seat 0")
	}
	if effects.MatchesPlayerSpecCtx(e.G, "Player.!IsTriggerRemembered", 1, 0, bound) {
		t.Error("Player.!IsTriggerRemembered admitted the captured seat 1")
	}
	for _, pc := range []effects.PlayerSpecCtx{{Source: src}, {}} {
		for _, p := range []state.PlayerID{0, 1} {
			if effects.MatchesPlayerSpecCtx(e.G, "Player.!IsTriggerRemembered", p, 0, pc) {
				t.Errorf("Player.!IsTriggerRemembered admitted seat %d without a capture", p)
			}
		}
	}
}
