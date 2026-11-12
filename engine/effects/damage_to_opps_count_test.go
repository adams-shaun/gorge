package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestDamageToOppsThisTurnCount pins Forge's DamageToOppsThisTurn player
// property on both spellings the corpus carries:
//
//   - TriggeredPlayer$DamageToOppsThisTurn (Antagonism's end-step gate): the
//     perspective is the FIRING TRIGGER's player, not the resolving
//     controller, and the value is the sum over that player's living
//     opponents of this turn's damage.
//   - PlayerCountPropertyYou$DamageToOppsThisTurn (Bloodcrazed Goblin's
//     CantAttack gate): the perspective is the RESOLVING controller.
//
// The empty fixture is a PRECONDITION: it must be an evaluated zero
// (ok == true), not the unresolved verdict a missing arm returns -- that is
// exactly the fake zero the end-step gate used to read.
func TestDamageToOppsThisTurnCount(t *testing.T) {
	type row struct {
		name       string
		seats      int
		dmg        map[state.PlayerID]int32
		controller state.PlayerID
		trigger    state.PlayerID
		want       int32
	}
	triggers := []row{
		{"empty is an evaluated zero", 2, nil, 0, 0, 0},
		{"own damage does not count", 2, map[state.PlayerID]int32{0: 5}, 0, 0, 0},
		{"the other seat's damage counts", 2, map[state.PlayerID]int32{1: 3}, 0, 0, 3},
		{
			"every other living seat sums", 3,
			map[state.PlayerID]int32{1: 2, 2: 4},
			0, 0, 6,
		},
		{
			"perspective is the triggered player, not the controller", 2,
			map[state.PlayerID]int32{0: 5},
			0, 1, 5,
		},
	}
	for _, tc := range triggers {
		t.Run("TriggeredPlayer/"+tc.name, func(t *testing.T) {
			h := newHost(t, tc.seats)
			h.dmgTaken = tc.dmg
			c := &Ctx{Controller: tc.controller,
				TriggerContext: TriggerContext{TriggerPlayer: state.Target{Player: tc.trigger, IsPlayer: true}}}
			got, ok := EvalCountOK(h, c, "TriggeredPlayer$DamageToOppsThisTurn")
			if !ok {
				t.Fatalf("TriggeredPlayer$DamageToOppsThisTurn is not evaluated (the fake-zero verdict)")
			}
			if got != tc.want {
				t.Fatalf("TriggeredPlayer$DamageToOppsThisTurn = %d, want %d", got, tc.want)
			}
		})
	}

	propertyYou := []row{
		{"empty is an evaluated zero", 2, nil, 0, 0, 0},
		{"own damage does not count", 2, map[state.PlayerID]int32{0: 5}, 0, 0, 0},
		{"the other seat's damage counts", 2, map[state.PlayerID]int32{1: 3}, 0, 0, 3},
		{
			"every other living seat sums", 3,
			map[state.PlayerID]int32{1: 2, 2: 4},
			0, 0, 6,
		},
		{
			"perspective is the controller", 2,
			map[state.PlayerID]int32{0: 5},
			1, 0, 5,
		},
		{
			"controller's own damage is excluded in the other seat", 2,
			map[state.PlayerID]int32{1: 3},
			1, 0, 0,
		},
	}
	for _, tc := range propertyYou {
		t.Run("PlayerCountPropertyYou/"+tc.name, func(t *testing.T) {
			h := newHost(t, tc.seats)
			h.dmgTaken = tc.dmg
			c := &Ctx{Controller: tc.controller,
				TriggerContext: TriggerContext{TriggerPlayer: state.Target{Player: tc.trigger, IsPlayer: true}}}
			got, ok := EvalCountOK(h, c, "PlayerCountPropertyYou$DamageToOppsThisTurn")
			if !ok {
				t.Fatalf("PlayerCountPropertyYou$DamageToOppsThisTurn is not evaluated")
			}
			if got != tc.want {
				t.Fatalf("PlayerCountPropertyYou$DamageToOppsThisTurn = %d, want %d", got, tc.want)
			}
		})
	}

	// Antagonism also carries SVar:Check:PlayerCountOpponents$
	// DamageToOppsThisTurn, but only its AI hint reads it: the group spelling
	// must stay unmodelled (and unlisted), so a rules gate over it fails open
	// rather than enforcing a fake sum.
	h := newHost(t, 3)
	h.dmgTaken = map[state.PlayerID]int32{1: 3, 2: 4}
	c := &Ctx{Controller: 0}
	if got, ok := EvalCountOK(h, c, "PlayerCountOpponents$DamageToOppsThisTurn"); ok || got != 0 {
		t.Fatalf("PlayerCountOpponents$DamageToOppsThisTurn = (%d, %v), want the unresolved (0, false)", got, ok)
	}
}
