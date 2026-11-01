package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// flashThompsonBothModes casts Flash Thompson and chooses both modes of its
// enters trigger, Heckle tapping `tap` and Hero Worship untapping `untap`,
// leaving the trigger on top of the stack.
func flashThompsonBothModes(t *testing.T, e *Engine, reg *cards.Registry, tap, untap state.ObjID) state.ObjID {
	t.Helper()
	castSpellNamed(t, e, reg, "Flash Thompson, Spider-Fan", "1W")
	d := passUntilAskKind(t, e, decision.KModes, 20)
	submitChoices(t, e, d.Options[0].Index, d.Options[1].Index)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("mode target ask: pending %+v", d)
	}
	// One ask with one option group per mode, in chosen order.
	first := d.Options[0].Group
	pick := []int{-1, -1}
	for _, o := range d.Options {
		if o.Group == first && o.Obj == tap {
			pick[0] = o.Index
		}
		if o.Group != first && o.Obj == untap {
			pick[1] = o.Index
		}
	}
	if pick[0] < 0 || pick[1] < 0 {
		t.Fatalf("mode target options: %+v", d.Options)
	}
	submitChoices(t, e, pick...)
	return e.G.Stack[len(e.G.Stack)-1]
}

// TestCopyOfTwoTargetCharmKeepsEachModesTargets: CR 707.10 copies modes and
// targets. A copy of "tap target creature; untap target creature" taps the
// first creature and untaps the second, as the original does, rather than
// running both modes over both targets.
func TestCopyOfTwoTargetCharmKeepsEachModesTargets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name  string
		elect bool
	}{{"keeps its targets", false}, {"elects to keep them", true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := searchTestRegistry(t)
			e, _ := searchEngine(t, reg, "Flash Thompson, Spider-Fan", "Serra Angel")
			bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
			angel := searchMoveByName(t, e, "Serra Angel", state.ZBattlefield)
			e.emit(events.Event{Kind: events.Tap, Obj: angel})
			e.priorityRound()

			trig := flashThompsonBothModes(t, e, reg, bear, angel)
			amount := int32(0)
			if tc.elect {
				amount = 1
			}
			e.emit(events.Event{Kind: events.StackCopy, Obj: trig, Player: 0, Amount: amount})
			e.priorityRound()
			copyID := e.G.Stack[len(e.G.Stack)-1]
			if copyID == trig {
				t.Fatalf("no copy on the stack: %v", e.G.Stack)
			}

			// The copy resolves first. Keeping each declaration's inherited
			// target answers an electing copy.
			for i := 0; i < 8 && e.G.Obj(copyID).Zone == state.ZStack; i++ {
				if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
					submitChoices(t, e, d.Options[0].Index)
					continue
				}
				passOnceP(t, e)
			}
			if e.G.Obj(copyID).Zone == state.ZStack {
				t.Fatalf("copy still on the stack, pending %+v", e.Pending())
			}
			if !e.G.Obj(bear).Tapped || e.G.Obj(angel).Tapped {
				t.Errorf("after the copy: bear tapped=%v (want true), angel tapped=%v (want false)",
					e.G.Obj(bear).Tapped, e.G.Obj(angel).Tapped)
			}
		})
	}
}
