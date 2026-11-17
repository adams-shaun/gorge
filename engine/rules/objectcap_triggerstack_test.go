package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestObjectCapIgnoresThePendingTriggerStack replays seed 78391's shape at
// fixture scale: the arena is far past the cap, but 500 pending CR 113.7a
// trigger objects are on the stack while the card-shaped board is under it.
// The watcher must not abort (the triggers are not board state); only the
// board crossing the cap does.
func TestObjectCapIgnoresThePendingTriggerStack(t *testing.T) {
	e := livelockTestEngine(t, &LoopGuard{MaxObjs: 100})
	stageObjs(t, e, 90, 0)
	for i := 0; i < 500; i++ {
		a := e.G.AddObject(nil, 0)
		e.emit(events.Event{Kind: events.MoveZone, Obj: a.ID, From: state.ZLibrary, To: state.ZStack})
		a.Ability = &cards.SA{}
	}
	// Precondition: the arena is past the cap, the board is not.
	if got := len(e.G.Objs); got != 590 {
		t.Fatalf("arena = %d, want 590 (90 staged cards + 500 pending triggers)", got)
	}
	if got, ref := e.G.LiveObjs(), countedLive(e.G); got != 90 || ref != 90 {
		t.Fatalf("live board = %d (reference %d), want 90 under cap 100", got, ref)
	}
	if lle := drivePattern(t, e, func(i int) events.Event {
		return events.Event{Kind: events.Note, Obj: state.ObjID(1 + i%3), Text: "storm resolving"}
	}, 5); lle != nil {
		t.Fatalf("watcher aborted on an arena past the cap whose board is 90: %v", lle)
	}
	// The board itself crossing the cap still aborts.
	stageObjs(t, e, 11, 0)
	var lle *LivelockError
	func() {
		defer func() {
			if r := recover(); r != nil {
				l, ok := r.(*LivelockError)
				if !ok {
					panic(r)
				}
				lle = l
			}
		}()
		e.emit(events.Event{Kind: events.Note, Obj: 1, Text: "board crossing"})
	}()
	if lle == nil || lle.Count != 101 || lle.Cap != 100 {
		t.Fatalf("board crossing: got %+v, want an object-cap abort at 101/100", lle)
	}
}
