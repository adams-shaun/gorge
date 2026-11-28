package effects

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// TestUnlessPayerDeparted is CR 800.4f at the UnlessPayer$ reader: a player
// who left the game is no payer. A selector that named only departed players
// resolves to nobody (known=false, the gate's decline), including the
// target-derived family whose ABSENT target is charged to the controller --
// the controller must not stand in for a departed payer.
func TestUnlessPayerDeparted(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	g := state.NewGame([]string{"you", "gone", "them"})
	h := &fakeHost{g: g}
	spell := corpusObjectInSeat(t, reg, g, "Grizzly Bears", 1, state.ZStack).ID
	h.Emit(events.Event{Kind: events.PlayerLost, Player: 1, Text: "test"})
	if !g.Players[1].Lost || g.Players[0].Lost || g.Players[2].Lost {
		t.Fatal("precondition: exactly seat 1 must have left the game")
	}
	if o := g.Obj(spell); o == nil || o.Controller != 1 {
		t.Fatal("precondition: the triggering source must be controlled by seat 1")
	}
	gone := state.Target{Player: 1, IsPlayer: true}
	live := state.Target{Player: 2, IsPlayer: true}

	for _, tc := range []struct {
		name, spec string
		in         CtxInit
		want       []state.PlayerID
		known      bool
	}{
		{name: "triggered activator", spec: "TriggeredActivator",
			in: CtxInit{TriggerContext: TriggerContext{TriggerActivator: gone}}},
		{name: "targeted controller", spec: "TargetedController",
			in: CtxInit{Targets: []state.Target{gone}}},
		{name: "default selector", spec: "",
			in: CtxInit{Targets: []state.Target{gone}}},
		{name: "triggered source controller", spec: "TriggeredSourceSAController",
			in: CtxInit{TriggerContext: TriggerContext{TriggerSource: spell}}},
		{name: "live player stays", spec: "TargetedPlayer",
			in:   CtxInit{Targets: []state.Target{gone, live}},
			want: []state.PlayerID{2}, known: true},
		// No target at all is not a departure: the controller fallback's
		// known-but-empty answer is unchanged.
		{name: "absent target", spec: "TargetedController", known: true},
	} {
		c := NewCtxPtr(0, 0, tc.in)
		ts, known := unlessPayerTargets(h, c, &cards.SA{Params: map[string]string{"UnlessPayer": tc.spec}})
		var got []state.PlayerID
		for _, p := range ts {
			got = append(got, p.Player)
		}
		if known != tc.known || !slices.Equal(got, tc.want) {
			t.Errorf("%s (UnlessPayer$ %q) = %v known=%v, want %v known=%v", tc.name, tc.spec, got, known, tc.want, tc.known)
		}
	}
}
