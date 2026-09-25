package events

import (
	"testing"

	"github.com/adams-shaun/gorge/state"
)

// TestStackCopyCarriesTheFaceDownMarker is the face-down half of StackCopy's
// state transfer (CR 708.4): a copy of a face-down, morph-family spell is
// itself face down, while an ordinary face-up spell's copy stays unmarked.
//
// The original's FaceDown bit is folded by Apply's MoveZone branch from the
// face-down entry Counter that rules/cast.go's pushCast rides on the
// PutOnStack. A copy is minted by AddObject and never passes through that
// fold, so StackCopy must derive the marker from the morph-family CastFlags
// it inherits.
func TestStackCopyCarriesTheFaceDownMarker(t *testing.T) {
	g, id := gameWithOneCard(t)
	// Set up exactly the state a face-down morph cast leaves: on the stack,
	// with the morph cast flag and the folded face-down marker.
	Apply(g, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack, Counter: FaceDownEntryCounter})
	if src := g.Obj(id); src == nil || !src.FaceDown {
		t.Fatalf("precondition: source not face down after PutOnStack: %+v", src)
	}
	// A morph cast's pay-time CastInfo carries the flag (rules/cast.go's
	// morph flag). Set it through the event path the engine uses.
	Apply(g, Event{Kind: CastInfo, Obj: id, Counter: FlagsString(state.FlagMorphed)})
	if src := g.Obj(id); src == nil || src.CastFlags&state.FlagMorphed == 0 {
		t.Fatalf("precondition: source lacks FlagMorphed: %+v", src)
	}

	Apply(g, Event{Kind: StackCopy, Obj: id, Player: 0})
	if len(g.Stack) != 2 {
		t.Fatalf("precondition: stack %v, want the original and its copy", g.Stack)
	}
	c := g.Obj(g.Stack[1])
	if c == nil || !c.IsCopy {
		t.Fatalf("precondition: top of stack is not a copy: %+v", c)
	}
	if !c.FaceDown {
		t.Errorf("copy of a face-down morph spell is not face down: %+v", c)
	}
	// The cloak state bit rides the Disguise variant only.
	if c.Cloaked {
		t.Errorf("a morph (non-disguise) copy must not be Cloaked: %+v", c)
	}

	// Control: an ordinary face-up spell copy inherits no morph flag and so
	// carries no face-down marker.
	g2, id2 := gameWithOneCard(t)
	Apply(g2, Event{Kind: PutOnStack, Obj: id2, Player: 0, From: state.ZHand, To: state.ZStack})
	Apply(g2, Event{Kind: StackCopy, Obj: id2, Player: 0})
	if len(g2.Stack) != 2 {
		t.Fatalf("control: stack %v", g2.Stack)
	}
	c2 := g2.Obj(g2.Stack[1])
	if c2 == nil || c2.FaceDown {
		t.Fatalf("control: ordinary spell copy is face down: %+v", c2)
	}
}

// TestStackCopyCloakedDisguiseCopy covers the Disguise variant of the
// face-down family: the copy inherits FlagDisguised, so it is face down and
// Cloaked, exactly as the original stack object folded from the cloak entry
// marker.
func TestStackCopyCloakedDisguiseCopy(t *testing.T) {
	g, id := gameWithOneCard(t)
	Apply(g, Event{Kind: PutOnStack, Obj: id, Player: 0, From: state.ZHand, To: state.ZStack, Counter: CloakEntryCounter})
	if src := g.Obj(id); src == nil || !src.FaceDown || !src.Cloaked {
		t.Fatalf("precondition: source not cloaked face down after PutOnStack: %+v", src)
	}
	Apply(g, Event{Kind: CastInfo, Obj: id, Counter: FlagsString(state.FlagDisguised)})
	if src := g.Obj(id); src == nil || src.CastFlags&state.FlagDisguised == 0 {
		t.Fatalf("precondition: source lacks FlagDisguised: %+v", src)
	}
	Apply(g, Event{Kind: StackCopy, Obj: id, Player: 0})
	if len(g.Stack) != 2 {
		t.Fatalf("precondition: stack %v", g.Stack)
	}
	c := g.Obj(g.Stack[1])
	if c == nil || !c.IsCopy {
		t.Fatalf("precondition: top of stack is not a copy: %+v", c)
	}
	if !c.FaceDown || !c.Cloaked {
		t.Errorf("disguise copy FaceDown=%v Cloaked=%v, want both true", c.FaceDown, c.Cloaked)
	}
}
