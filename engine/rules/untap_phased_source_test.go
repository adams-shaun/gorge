package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// seedbornMuseScript is a hand-written fixture of the card's static shape.
const seedbornMuseScript = "Name:Seedborn Muse\nManaCost:3 G G\nTypes:Creature Spirit\nPT:2/4\n" +
	"S:Mode$ UntapOtherPlayer | ValidCard$ Permanent.YouCtrl\nOracle:x\n"

// untapOtherSourceCase runs seat 0's untap step over seat 1's Seedborn Muse
// and tapped Mountain, with hide applied to the Muse first, and returns
// whether the Mountain is still tapped.
func untapOtherSourceCase(t *testing.T, hide func(e *Engine, muse state.ObjID)) bool {
	t.Helper()
	e := handEngine(t)
	muse := onBoard(t, e, 1, seedbornMuseScript)
	land := onBoard(t, e, 1, mountainScript())
	e.emit(events.Event{Kind: events.Tap, Obj: land})
	if !e.G.Obj(land).Tapped {
		t.Fatal("precondition: the Mountain did not tap")
	}
	if hide != nil {
		hide(e, muse)
	}
	e.G.Turn = 2
	e.beginTurn(0)
	return e.G.Obj(land).Tapped
}

// TestPhasedOutUntapOtherStaticDoesNotUntap pins the UntapOtherPlayer source
// gate: a phased-out Seedborn Muse is treated as though it does not exist (CR
// 702.26b) and a face-down one has no abilities (CR 708.2), so either way its
// controller's permanents stay tapped during the other player's untap step.
// The control case shows the same board untaps with the Muse face up and
// phased in, so the assertions can fail.
func TestPhasedOutUntapOtherStaticDoesNotUntap(t *testing.T) {
	t.Parallel()
	if untapOtherSourceCase(t, nil) {
		t.Fatal("control: a phased-in, face-up Seedborn Muse did not untap its controller's Mountain")
	}
	phased := func(e *Engine, muse state.ObjID) {
		e.emit(events.Event{Kind: events.PhaseOut, Obj: muse, Amount: 1})
		if !e.G.Obj(muse).PhasedOut {
			t.Fatal("precondition: Seedborn Muse did not phase out")
		}
	}
	if !untapOtherSourceCase(t, phased) {
		t.Fatal("a phased-out Seedborn Muse untapped its controller's Mountain (CR 702.26b)")
	}
	faceDown := func(e *Engine, muse state.ObjID) {
		e.G.Obj(muse).FaceDown = true
		if !e.faceDownPrintedHides(e.G.Obj(muse)) {
			t.Fatal("precondition: the face-down Seedborn Muse's printed statics are not hidden")
		}
	}
	if !untapOtherSourceCase(t, faceDown) {
		t.Fatal("a face-down Seedborn Muse untapped its controller's Mountain (CR 708.2)")
	}
}
