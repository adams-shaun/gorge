package view

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/state"
)

// TestZoneListsSkipEphemeralObjects is Task 4's regression test for
// cardViews' ephemeral skip: a token that has left the battlefield (this
// build parks such an object in exile rather than deleting it -- see
// state.Object.Ephemeral) and a copy sitting in exile must not appear in
// any zone view, while a token genuinely on the battlefield still does.
//
// This is a self-contained fixture (its own two-seat game and card) rather
// than a parallel M2a task's twoSeatWith/watcherSrc helpers, which do not
// exist on this branch (see Task 4's coordination note).
func TestZoneListsSkipEphemeralObjects(t *testing.T) {
	g := state.NewGame([]string{"a", "b"})
	bear, d := cards.ParseBytes("b.txt", []byte("Name:Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	bear.Link()

	// A token still on the battlefield: must appear.
	live := g.AddObject(bear, 0)
	live.IsToken = true
	live.Zone = state.ZBattlefield
	g.SetZone(state.ZBattlefield, 0, []state.ObjID{live.ID})

	// A token that has left the battlefield, parked in exile: must not
	// appear.
	deadToken := g.AddObject(bear, 0)
	deadToken.IsToken = true
	deadToken.Zone = state.ZExile
	// A copy sitting in exile: must not appear either.
	deadCopy := g.AddObject(bear, 0)
	deadCopy.IsCopy = true
	deadCopy.Zone = state.ZExile
	g.SetZone(state.ZExile, 0, []state.ObjID{deadToken.ID, deadCopy.ID})

	v := Project(g, nil, 0, nil)
	bf := v.Players[0].Battlefield
	if len(bf) != 1 || bf[0].ID != live.ID {
		t.Fatalf("battlefield = %+v, want only the live token %d", bf, live.ID)
	}
	if ex := v.Players[0].Exile; len(ex) != 0 {
		t.Fatalf("exile = %+v, want ephemeral objects skipped", ex)
	}
}
