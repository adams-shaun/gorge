package view

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// TestFaceDownStackCopyIsRedactedFromOpponents is the view-side regression
// for CR 708.4 on a COPY of a face-down spell. A copy is minted by
// events.Apply's StackCopy case, which now derives the FaceDown marker from
// the morph-family CastFlags it inherits; this test drives the real event
// path and asserts the redaction consumer (stackViews) actually hides the
// copy's printed identity from an opponent and shows it to its controller.
func TestFaceDownStackCopyIsRedactedFromOpponents(t *testing.T) {
	g, id := viewTestGame(t)
	// Put a face-down morph spell on the stack, there set its cast flag, and
	// copy it -- exactly the state a real cast leaves.
	events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0,
		From: state.ZHand, To: state.ZStack, Counter: events.FaceDownEntryCounter})
	events.Apply(g, events.Event{Kind: events.CastInfo, Obj: id, Counter: events.FlagsString(state.FlagMorphed)})
	events.Apply(g, events.Event{Kind: events.StackCopy, Obj: id, Player: 0})

	if len(g.Stack) != 2 {
		t.Fatalf("precondition: stack %v, want the spell and its copy", g.Stack)
	}
	copyID := g.Stack[1]
	copyObj := g.Obj(copyID)
	if copyObj == nil || !copyObj.IsCopy {
		t.Fatalf("precondition: top of stack is not a copy: %+v", copyObj)
	}
	if !copyObj.FaceDown {
		t.Fatalf("precondition: the copy is not face down, so redaction cannot be exercised")
	}

	ch := flatChars{g}
	// Opponent's (seat 1) seat view: the copied spell must carry no name,
	// text or card.
	opp := project(g, ch, 1, nil, false)
	var oppCopy *StackView
	for i := range opp.Stack {
		if opp.Stack[i].ID == copyID {
			oppCopy = &opp.Stack[i]
		}
	}
	if oppCopy == nil {
		t.Fatalf("precondition: copy %d absent from opponent's stack view %+v", copyID, opp.Stack)
	}
	if oppCopy.Name != "" || oppCopy.Text != "" || oppCopy.Card != nil {
		t.Errorf("opponent sees a copied face-down spell's identity: name=%q text=%q card=%v",
			oppCopy.Name, oppCopy.Text, oppCopy.Card != nil)
	}

	// Controller's (seat 0) seat view: the printed band is visible.
	ctrl := project(g, ch, 0, nil, false)
	var ctrlCopy *StackView
	for i := range ctrl.Stack {
		if ctrl.Stack[i].ID == copyID {
			ctrlCopy = &ctrl.Stack[i]
		}
	}
	if ctrlCopy == nil {
		t.Fatalf("precondition: copy %d absent from controller's stack view %+v", copyID, ctrl.Stack)
	}
	if ctrlCopy.Name == "" || ctrlCopy.Card == nil {
		t.Errorf("controller cannot see their own copied face-down spell: name=%q card=%v",
			ctrlCopy.Name, ctrlCopy.Card != nil)
	}
}

// TestOrdinaryStackCopyIsNotRedacted is the control: a face-up spell's copy
// inherits no morph flag, so its projected name and card are visible to
// opponents just as the original's are.
func TestOrdinaryStackCopyIsNotRedacted(t *testing.T) {
	g, id := viewTestGame(t)
	events.Apply(g, events.Event{Kind: events.PutOnStack, Obj: id, Player: 0,
		From: state.ZHand, To: state.ZStack})
	events.Apply(g, events.Event{Kind: events.StackCopy, Obj: id, Player: 0})

	if len(g.Stack) != 2 {
		t.Fatalf("precondition: stack %v", g.Stack)
	}
	copyID := g.Stack[1]
	if c := g.Obj(copyID); c == nil || !c.IsCopy || c.FaceDown {
		t.Fatalf("precondition: not a face-up copy: %+v", c)
	}
	opp := project(g, flatChars{g}, 1, nil, false)
	var oppCopy *StackView
	for i := range opp.Stack {
		if opp.Stack[i].ID == copyID {
			oppCopy = &opp.Stack[i]
		}
	}
	if oppCopy == nil {
		t.Fatalf("precondition: copy absent from opponent's stack view")
	}
	if oppCopy.Name == "" || oppCopy.Card == nil {
		t.Errorf("ordinary copy was redacted: name=%q card=%v", oppCopy.Name, oppCopy.Card != nil)
	}
}

// viewTestGame builds a two-seat game with one real card in seat 0's hand.
func viewTestGame(t *testing.T) (*state.Game, state.ObjID) {
	t.Helper()
	g := state.NewGame([]string{"Ann", "Bob"})
	c, d := cards.ParseBytes("morph.txt", []byte(
		"Name:Test Morph\nManaCost:3\nTypes:Creature Bear\nPT:2/2\nOracle:x\nK:Morph:2 U\n"))
	if len(d) != 0 {
		t.Fatalf("diags: %v", d)
	}
	c.Link()
	o := g.AddObject(c, 0)
	o.Zone = state.ZHand
	g.SetZone(state.ZHand, 0, []state.ObjID{o.ID})
	return g, o.ID
}
