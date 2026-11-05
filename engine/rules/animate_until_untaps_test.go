package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// untapsAnimateBoard puts a Duration$ UntilUntaps Animate host (Hedge
// Whisperer's shape) and a land on the battlefield and resolves the host's
// ability against the land.
func untapsAnimateBoard(t *testing.T, tapHost bool) (*Engine, state.ObjID, state.ObjID) {
	t.Helper()
	e := layerEngine(t)
	host := onBoard(t, e, 0, "Name:Untap Whisperer\nTypes:Creature Elf\nPT:0/3\nA:AB$ Animate | Cost$ T | Defined$ Targeted | Duration$ UntilUntaps | Power$ 5 | Toughness$ 5 | Types$ Creature,Plant,Boar | Keywords$ Haste | Colors$ Green | OverwriteColors$ True\nOracle:x\n")
	land := onBoard(t, e, 0, "Name:Untap Field\nTypes:Land\nOracle:x\n")
	if tapHost {
		e.emit(events.Event{Kind: events.Tap, Obj: host})
	}
	face := e.G.Obj(host).Face()
	effects.Resolve(e, &effects.Ctx{Source: host, Controller: 0, SVars: face.SVars,
		Targets: []state.Target{{Obj: land}}}, face.Abilities[0])
	return e, host, land
}

func animatedBoar(e *Engine, id state.ObjID) bool {
	d := e.Derived(id)
	return d.Power == 5 && d.Toughness == 5 && e.IsCreature(id)
}

func TestAnimateUntilUntapsDuration(t *testing.T) {
	t.Parallel()
	t.Run("survives cleanup while the host stays tapped", func(t *testing.T) {
		e, host, land := untapsAnimateBoard(t, true)
		if !e.G.Obj(host).Tapped || !animatedBoar(e, land) {
			t.Fatalf("precondition: host tapped=%v, land animated=%v", e.G.Obj(host).Tapped, animatedBoar(e, land))
		}
		e.EndOfTurnCleanup()
		if !animatedBoar(e, land) {
			t.Fatal("after cleanup the land is no 5/5 creature (UntilUntaps ended at end of turn)")
		}
	})
	t.Run("ends when the host untaps and a retap does not revive it", func(t *testing.T) {
		e, host, land := untapsAnimateBoard(t, true)
		e.EndOfTurnCleanup()
		e.emit(events.Event{Kind: events.Untap, Obj: host})
		if e.IsCreature(land) {
			t.Fatal("after the host untapped the land is still a creature")
		}
		e.emit(events.Event{Kind: events.Tap, Obj: host})
		if e.IsCreature(land) {
			t.Fatal("after a retap the land is a creature again (effect revived)")
		}
	})
	t.Run("ends when the host leaves the battlefield", func(t *testing.T) {
		e, host, land := untapsAnimateBoard(t, true)
		e.emit(events.Event{Kind: events.MoveZone, Obj: host, From: state.ZBattlefield, To: state.ZGraveyard, Player: 0})
		if e.G.Obj(host).Zone == state.ZBattlefield {
			t.Fatal("precondition: host still on the battlefield")
		}
		if e.IsCreature(land) {
			t.Fatal("after the host left the land is still a creature")
		}
	})
	t.Run("an untapped host animates nothing", func(t *testing.T) {
		e, _, land := untapsAnimateBoard(t, false)
		if e.IsCreature(land) {
			t.Fatal("untapped-host Animate made the land a creature (Forge checkValidDuration)")
		}
	})
}
