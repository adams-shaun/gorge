package events

import (
	"reflect"
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// The stamped stack kind (state.Object.StackKind/StackKindKnown,
// state/stackkind.go) is a property of STACK MEMBERSHIP: every mint stamps
// it, leaving the stack must un-stamp it, or a CR 733.1 abort reversal (whose
// MoveZone returns the card to the zone it came from) restores an object that
// no longer matches its pre-push state byte for byte -- the exact shape the
// cr733_abort_conformance oracle caught on Lightning Bolt/Terminus.
func TestLeavingStackUnstampsStackKind(t *testing.T) {
	g, id := gameWithOneCardSrc(t, "Name:Bolt\nManaCost:1 R\nTypes:Instant\nOracle:x\n")
	before := g.Clone()

	// Precondition: a fresh object in hand carries no stamp.
	if g.Obj(id).StackKindKnown {
		t.Fatalf("hand card pre-stamped: kind=%d known=%t", g.Obj(id).StackKind, g.Obj(id).StackKindKnown)
	}

	// Cast push: the card enters the stack and the fold stamps it a spell.
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZHand, To: state.ZStack})
	if got := g.Obj(id); !got.StackKindKnown || got.StackKind != state.StackKindSpell {
		t.Fatalf("precondition failed: stack card not stamped: kind=%d known=%t", got.StackKind, got.StackKindKnown)
	}

	// The CR 733.1 reversal exactly as abortCast emits it: MoveZone back to
	// the hand, Text "reversed", which also restores the pre-push entry
	// provenance. The fold must un-stamp, restoring the object
	// byte-identically to `before`.
	Apply(g, Event{Kind: MoveZone, Obj: id, From: state.ZStack, To: state.ZHand, Text: "reversed"})
	got := g.Obj(id)
	if got.StackKindKnown {
		t.Fatalf("hand card kept its stamp after leaving the stack: kind=%d known=%t", got.StackKind, got.StackKindKnown)
	}
	want := before.Obj(id)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("reversed push did not restore the object byte-identically:\nbefore %+v\nafter  %+v", *want, *got)
	}
}

// Same membership rule for an ability object: AbilityPush stamps its mint
// Activated, and the reversal's MoveZone (stack -> exile) un-stamps it.
func TestAbilityLeavingStackUnstampsStackKind(t *testing.T) {
	g, id := gameWithOneCardSrc(t, "Name:Sailor\nManaCost:U\nTypes:Creature Spirit\nPT:1/1\nA:AB$ Draw | Cost$ 3 U | NumCards$ 1 | Defined$ You | SpellDescription$ Draw a card.\nOracle:x\n")
	Move(g, id, state.ZHand, state.ZBattlefield)
	Apply(g, Event{Kind: AbilityPush, Obj: id, Player: 0, Amount: 0})
	if len(g.Stack) != 1 {
		t.Fatal("precondition failed: no ability object on the stack")
	}
	ab := g.Obj(g.Stack[0])
	if !ab.StackKindKnown || ab.StackKind != state.StackKindActivated {
		t.Fatalf("precondition failed: ability mint kind=%d known=%t", ab.StackKind, ab.StackKindKnown)
	}
	abID := ab.ID

	// abortCast's ability reversal: MoveZone stack -> exile ("reversed").
	// The reversal consumes the mint's transient PreStackEntry fields (the
	// "reversed" protocol), so the assertion here is the stamp itself: an
	// exiled ability object is no longer a stack object and carries no kind.
	Apply(g, Event{Kind: MoveZone, Obj: abID, From: state.ZStack, To: state.ZExile, Text: "reversed"})
	got := g.Obj(abID)
	if got.StackKindKnown {
		t.Fatalf("exiled ability object kept its stamp: kind=%d known=%t", got.StackKind, got.StackKindKnown)
	}
}
