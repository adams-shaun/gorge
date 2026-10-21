package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// untapsPumpBoard puts a Duration$ UntilUntaps pump host and a 2/2 target on
// the battlefield and resolves the host's ability against the target.
func untapsPumpBoard(t *testing.T, tapHost bool) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	host := onBoard(t, e, 0, "Name:Untap Sword\nTypes:Artifact\nA:AB$ Pump | Cost$ T | Defined$ Targeted | NumAtt$ +2 | NumDef$ +0 | KW$ Trample | Duration$ UntilUntaps\nOracle:x\n")
	target := onBoard(t, e, 0, "Name:Untap Bear\nTypes:Creature Bear\nPT:2/2\nOracle:x\n")
	if tapHost {
		e.emit(events.Event{Kind: events.Tap, Obj: host})
	}
	face := e.G.Obj(host).Face()
	effects.Resolve(e, &effects.Ctx{Source: host, Controller: 0, SVars: face.SVars,
		Targets: []state.Target{{Obj: target}}}, face.Abilities[0])
	return e, host, target
}

func TestUntilUntapsDuration(t *testing.T) {
	t.Parallel()
	t.Run("survives cleanup while the host stays tapped", func(t *testing.T) {
		e, host, target := untapsPumpBoard(t, true)
		if !e.G.Obj(host).Tapped || e.Derived(target).Power != 4 {
			t.Fatalf("precondition: host tapped=%v power=%d, want tapped 4", e.G.Obj(host).Tapped, e.Derived(target).Power)
		}
		e.EndOfTurnCleanup()
		if d := e.Derived(target); d.Power != 4 {
			t.Fatalf("after cleanup power = %d, want 4 (UntilUntaps ended at end of turn)", d.Power)
		}
	})
	t.Run("ends when the host untaps and a retap does not revive it", func(t *testing.T) {
		e, host, target := untapsPumpBoard(t, true)
		e.EndOfTurnCleanup()
		e.emit(events.Event{Kind: events.Untap, Obj: host})
		if d := e.Derived(target); d.Power != 2 {
			t.Fatalf("after untap power = %d, want 2", d.Power)
		}
		e.emit(events.Event{Kind: events.Tap, Obj: host})
		if d := e.Derived(target); d.Power != 2 {
			t.Fatalf("after retap power = %d, want 2 (effect revived)", d.Power)
		}
	})
	t.Run("ends when the host leaves the battlefield", func(t *testing.T) {
		e, host, target := untapsPumpBoard(t, true)
		e.emit(events.Event{Kind: events.MoveZone, Obj: host, From: state.ZBattlefield, To: state.ZGraveyard, Player: 0})
		if e.G.Obj(host).Zone == state.ZBattlefield {
			t.Fatal("precondition: host still on the battlefield")
		}
		if d := e.Derived(target); d.Power != 2 {
			t.Fatalf("after host left power = %d, want 2", d.Power)
		}
	})
	t.Run("an untapped host applies nothing", func(t *testing.T) {
		e, _, target := untapsPumpBoard(t, false)
		if d := e.Derived(target); d.Power != 2 {
			t.Fatalf("untapped-host pump power = %d, want 2 (Forge checkValidDuration)", d.Power)
		}
	})
}
