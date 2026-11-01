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
	castSpellNamed(t, e, reg, "Flash Thompson, Spider-Fan", "WW")
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

// mobsterTriggerOnStack casts Mechanical Mobster with every graveyard empty, so
// its enters trigger's "up to one" root settles with no target, names `connive`
// for the chain link, and leaves the trigger on top of the stack.
func mobsterTriggerOnStack(t *testing.T, e *Engine, reg *cards.Registry, connive state.ObjID) state.ObjID {
	t.Helper()
	castSpellNamed(t, e, reg, "Mechanical Mobster", "WWW")
	for i := 0; i < 8; i++ {
		d := e.Pending()
		if d != nil && d.Kind == decision.KTarget {
			break
		}
		passOnceP(t, e)
	}
	d := e.Pending()
	if d == nil || d.Kind != decision.KTarget || d.ResumeKind != "trig_sub" {
		t.Fatalf("chain link ask: pending %+v", d)
	}
	for _, o := range d.Options {
		if o.Obj == connive {
			submitChoices(t, e, o.Index)
			return e.G.Stack[len(e.G.Stack)-1]
		}
	}
	t.Fatalf("no option for %d: %+v", connive, d.Options)
	return 0
}

// TestCopyOfAbilityKeepsItsChainTargets: CR 707.10 copies every target of the
// ability, its SubAbility$ links' included, and CR 707.10c lets an electing
// copy's controller keep or change them. A copy of Mechanical Mobster's trigger
// connives the creature the original named, or the one the election names, and
// resolves with no further target ask.
func TestCopyOfAbilityKeepsItsChainTargets(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name          string
		elect, change bool
	}{{"keeps its targets", false, false}, {"elects to keep them", true, false}, {"elects a new one", true, true}} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			reg := searchTestRegistry(t)
			e, _ := searchEngine(t, reg, "Mechanical Mobster", "Serra Angel")
			bear := searchMoveByName(t, e, "Grizzly Bears", state.ZBattlefield)
			angel := searchMoveByName(t, e, "Serra Angel", state.ZBattlefield)

			trig := mobsterTriggerOnStack(t, e, reg, bear)
			if subs := e.G.Obj(trig).SubTargets; len(subs) != 1 || subs[0].Obj != bear {
				t.Fatalf("original's chain targets: %+v", subs)
			}
			amount := int32(0)
			if tc.elect {
				amount = 1
			}
			e.emit(events.Event{Kind: events.StackCopy, Obj: trig, Player: 0, Amount: amount})
			e.priorityRound()
			copyID := e.G.Stack[len(e.G.Stack)-1]
			if subs := e.G.Obj(copyID).SubTargets; len(subs) != 1 || subs[0].Obj != bear {
				t.Fatalf("copy's chain targets on the stack: %+v", subs)
			}

			start := len(e.L.Events)
			want := bear
			for i := 0; i < 8 && e.G.Obj(copyID).Zone == state.ZStack; i++ {
				d := e.Pending()
				switch {
				case d != nil && d.Kind == decision.KPriority:
					passOnceP(t, e)
				case d != nil && d.Kind == decision.KTarget && d.ResumeKind == "copy_targets":
					if !tc.elect || d.Options[0].Obj != bear {
						t.Fatalf("copy_targets ask: %+v", d)
					}
					pick := d.Options[0].Index
					if tc.change {
						for _, o := range d.Options {
							if o.Obj == angel {
								pick, want = o.Index, angel
							}
						}
					}
					submitChoices(t, e, pick)
				case d != nil && d.Kind == decision.KTarget:
					t.Fatalf("resolution-time target ask on the copy: %+v", d)
				default:
					submitChoices(t, e, d.Options[0].Index) // the connive discard
				}
			}
			connived := false
			for _, ev := range e.L.Events[start:] {
				connived = connived || (ev.Kind == events.Connive && ev.Obj == want)
			}
			if !connived {
				t.Errorf("the copy did not connive %d", want)
			}
		})
	}
}
