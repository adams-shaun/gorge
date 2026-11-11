package events

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// TestExtraBlockRepointKeepsIsBlocking: a blocker of two attackers (CR 509.1a,
// CanBlockAmount$) keeps blocking when one of them is re-pointed, and stops
// only when the last attacker it blocks is re-pointed.
func TestExtraBlockRepointKeepsIsBlocking(t *testing.T) {
	g := state.NewGame([]string{"a", "b", "c"})
	l := NewLog(1)
	bear := bearCard()
	a := g.AddObject(bear, 0).ID
	b := g.AddObject(bear, 0).ID
	blk := g.AddObject(bear, 1).ID
	g.SetZone(state.ZLibrary, 0, []state.ObjID{a, b})
	g.SetZone(state.ZLibrary, 1, []state.ObjID{blk})
	for _, id := range []state.ObjID{a, b, blk} {
		Emit(g, l, Event{Kind: MoveZone, Obj: id, From: state.ZLibrary, To: state.ZBattlefield})
	}
	for _, id := range []state.ObjID{a, b} {
		g.Obj(id).IsAttacking = true
		g.Obj(id).Attacking = 1
	}
	Emit(g, l, Event{Kind: DeclareBlockers, Pairs: [][2]state.ObjID{{a, blk}, {b, blk}}})
	if !g.Obj(blk).IsBlocking || len(g.Obj(a).BlockedBy) != 1 || len(g.Obj(b).BlockedBy) != 1 {
		t.Fatalf("precondition: IsBlocking=%v A=%v B=%v", g.Obj(blk).IsBlocking, g.Obj(a).BlockedBy, g.Obj(b).BlockedBy)
	}
	Emit(g, l, Event{Kind: CombatRetarget, Obj: a, Player: 2})
	if !g.Obj(blk).IsBlocking {
		t.Fatal("blocker stopped blocking while it still blocks B")
	}
	if len(g.Obj(a).BlockedBy) != 0 || len(g.Obj(b).BlockedBy) != 1 || g.Obj(b).BlockedBy[0] != blk {
		t.Fatalf("BlockedBy A=%v B=%v, want A empty and B intact", g.Obj(a).BlockedBy, g.Obj(b).BlockedBy)
	}
	Emit(g, l, Event{Kind: CombatRetarget, Obj: b, Player: 2})
	if g.Obj(blk).IsBlocking {
		t.Fatal("blocker still blocking after its last attacker was re-pointed")
	}
}
