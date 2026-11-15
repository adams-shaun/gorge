package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestTriggeredPlayersAttackedTargetAmount pins the count head Commander
// Liara Portyr's SVar:X reads: the number of distinct players the firing
// AttackersDeclared declaration attacked, from the fire-time capture.
// Amount is the only property; a ctx with no capture reads zero.
func TestTriggeredPlayersAttackedTargetAmount(t *testing.T) {
	h := newHost(t, 4)
	const head = "TriggeredPlayersAttackedTarget$Amount"
	for _, tc := range []struct {
		name    string
		players []state.PlayerID
		want    int32
	}{
		{"one defender", []state.PlayerID{1}, 1},
		{"two defenders", []state.PlayerID{1, 3}, 2},
		{"three defenders", []state.PlayerID{1, 2, 3}, 3},
		{"no capture", nil, 0},
	} {
		c := &Ctx{Controller: 0}
		c.TriggerAttackedPlayers = tc.players
		n, ok := EvalCountOK(h, c, head)
		if !ok || n != tc.want {
			t.Errorf("%s: %s = %d ok=%v, want %d true", tc.name, head, n, ok, tc.want)
		}
	}
	c := &Ctx{Controller: 0}
	c.TriggerAttackedPlayers = []state.PlayerID{1, 3}
	if _, ok := EvalCountOK(h, c, "TriggeredPlayersAttackedTarget$LifeTotal"); ok {
		t.Errorf("TriggeredPlayersAttackedTarget$LifeTotal evaluated; only Amount is modelled")
	}
}
