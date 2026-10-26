package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestBlockedFilterProperty pins CR 509.1h's blocked attacker predicate
// (Forge's CardProperty `blocked`, Tactical Advantage's
// `Creature.blocked,Creature.blocking`): an attacking creature with a
// recorded blocker matches, an unblocked attacker does not, and the blocked
// attacker keeps matching after its blocker leaves combat (the zero
// tombstone preserves the BlockedBy list length).
func TestBlockedFilterProperty(t *testing.T) {
	g, ids := board(t)
	att := g.Obj(ids["theirBig"])
	unblocked := g.Obj(ids["myFlier"])
	blk := g.Obj(ids["myBear"])
	att.IsAttacking = true
	att.Attacking = 0
	unblocked.IsAttacking = true
	unblocked.Attacking = 0
	l := events.NewLog(1)

	// Precondition: before the declaration neither attacker is blocked, so
	// the assertions below cannot pass vacuously.
	if MatchesSpec(g, "Creature.blocked", att.ID, 0) {
		t.Fatal("precondition: attacker matched blocked before any block was declared")
	}
	if MatchesSpec(g, "Creature.blocked", unblocked.ID, 0) {
		t.Fatal("precondition: unblocked attacker matched blocked before any block was declared")
	}

	events.Emit(g, l, events.Event{Kind: events.DeclareBlockers, Pairs: [][2]state.ObjID{{att.ID, blk.ID}}})

	// Precondition: the pair is really recorded, so the blocked assertion
	// below reads a real blocker.
	if len(att.BlockedBy) != 1 || att.BlockedBy[0] != blk.ID || !blk.IsBlocking {
		t.Fatalf("precondition: attacker BlockedBy=%v blocker IsBlocking=%v", att.BlockedBy, blk.IsBlocking)
	}
	if !MatchesSpec(g, "Creature.blocked", att.ID, 0) {
		t.Fatal("blocked predicate false for an attacker with a recorded blocker")
	}
	if MatchesSpec(g, "Creature.blocked", unblocked.ID, 0) {
		t.Fatal("blocked predicate true for an unblocked attacker")
	}

	// The blocker leaves combat. BlockedBy keeps its length with a zero
	// tombstone, so the attacker is still blocked (CR 509.1h).
	events.Emit(g, l, events.Event{Kind: events.MoveZone, Obj: blk.ID, From: state.ZBattlefield, To: state.ZGraveyard})
	if len(att.BlockedBy) != 1 || att.BlockedBy[0] != 0 {
		t.Fatalf("precondition: BlockedBy = %v after the blocker left, want the zero tombstone", att.BlockedBy)
	}
	if !MatchesSpec(g, "Creature.blocked", att.ID, 0) {
		t.Fatal("blocked predicate false after the blocker was destroyed; the attacker must stay blocked")
	}
	if MatchesSpec(g, "Creature.blocked", unblocked.ID, 0) {
		t.Fatal("blocked predicate true for the unblocked attacker after the blocker left")
	}
}
