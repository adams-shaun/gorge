package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestNonTargetedControllerReadsTargetLKI pins Fractured Identity's new
// script: the targeted permanent is exiled by the parent ChangeZone before
// the CopyPermanent runs, so the controller to exclude is the one captured
// with the target at resolution start (CR 608.2h, 400.7), not the owner the
// live exiled card was reset to.
func TestNonTargetedControllerReadsTargetLKI(t *testing.T) {
	h := newHost(t, 4)
	card := mkCard(t, "Name:Fractured permanent\nTypes:Creature\nPT:3/3\nOracle:x\n")
	target := h.g.AddObject(card, 0) // owned by seat 0 ...
	target.Controller = 2            // ... controlled by seat 2 when targeted
	target.Zone = state.ZBattlefield
	// The parent ChangeZone exiles it: events.Apply's Move resets the live
	// controller to the owner, and the resolution-start snapshot keeps the
	// controller it had.
	target.Zone = state.ZExile
	target.Controller = target.Owner
	c := NewCtxPtr(0, 0, CtxInit{
		Targets: []state.Target{{Obj: target.ID}},
		Snap:    LKISnapshots{TargetController: map[state.ObjID]state.PlayerID{target.ID: 2}},
	})
	// Precondition: the live object and the snapshot disagree, so a read of
	// the live controller cannot pass this test.
	if got := h.g.Obj(target.ID).Controller; got != 0 {
		t.Fatalf("live controller = %d, want the owner 0 after the exile", got)
	}
	if got := c.Snap.TargetController[target.ID]; got != 2 {
		t.Fatalf("target snapshot controller = %d, want 2", got)
	}

	sa := &cards.SA{API: "CopyPermanent", Params: map[string]string{
		"Defined": "Targeted", "Controller": "NonTargetedController",
	}}
	effCopyPermanent(h, c, sa)

	counts := make([]int, len(h.g.Players))
	for i := range h.g.Objs {
		o := &h.g.Objs[i]
		if o.IsToken && o.Face() != nil && o.Face().Name == card.Faces[0].Name {
			counts[o.Controller]++
		}
	}
	if counts[0] != 1 || counts[1] != 1 || counts[2] != 0 || counts[3] != 1 {
		t.Fatalf("NonTargetedController copy owners = %v, want [1 1 0 1]; events=%+v", counts, h.log)
	}
	for _, ev := range h.log {
		if ev.Kind == events.Note {
			t.Fatalf("unexpected unsupported-controller note: %q", ev.Text)
		}
	}

	// The same selector through the repeat-player reader CopyPermanent's
	// multi-owner branch takes.
	players, ok := repeatPlayers(h, c, "NonTargetedController")
	if !ok || len(players) != 3 || players[0] != 0 || players[1] != 1 || players[2] != 3 {
		t.Fatalf("repeatPlayers NonTargetedController = %v/%v, want [0 1 3]", players, ok)
	}
}
