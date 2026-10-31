package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const (
	testWarden   = "Name:Test Warden\nManaCost:2\nTypes:Creature Human\nPT:2/2\nOracle:x\n"
	testSnare    = "Name:Test Snare\nManaCost:U\nTypes:Enchantment Aura\nK:Enchant:Creature\nOracle:x\n"
	testColossus = "Name:Test Colossus\nManaCost:5\nTypes:Creature Giant\nPT:5/5\nOracle:x\n"
	testLowTide  = "Name:Test Low Tide\nManaCost:W\nTypes:Sorcery\n" +
		"A:SP$ DestroyAll | ValidCards$ Creature.powerLE2 | SpellDescription$ Small creatures perish together.\nOracle:x\n"
)

// holdUntilHostLeaves puts src into exile as if host had exiled it from the
// battlefield until host leaves.
func holdUntilHostLeaves(t *testing.T, e *Engine, host state.ObjID, src string) state.ObjID {
	t.Helper()
	o := e.G.AddObject(card(t, src), 1)
	e.emit(events.Event{Kind: events.MoveZone, Obj: o.ID, From: state.ZLibrary, To: state.ZExile})
	e.emit(events.Event{Kind: events.Imprint, Obj: host, IDs: []state.ObjID{o.ID},
		Amount: int32(state.ZBattlefield), Text: "until-host-leaves"})
	return o.ID
}

func castLowTide(t *testing.T, e *Engine) {
	t.Helper()
	e.G.Players[0].Pool[state.MW] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	for i := 0; i < 8 && len(e.G.Stack) > 0; i++ {
		d := e.Pending()
		if d == nil || d.Kind != decision.KPriority {
			t.Fatalf("pending %+v with %d on the stack, want only priority: nothing about the batch is a choice", d, len(e.G.Stack))
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{passPriorityOption(t, d)}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
}

// CR 610.3 returns the exiled card immediately after its host leaves, and a
// destroy-all is one event: every creature it destroys has already gone when
// the Aura comes back, so it has nothing to enchant (CR 303.4g).
func TestCR610_3AuraReturnFindsTheBoardAfterTheDestroyAll(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, testLowTide))
	warden := onBoard(t, e, 0, testWarden)
	bear := onBoard(t, e, 1, testWarden)
	aura := holdUntilHostLeaves(t, e, warden, testSnare)

	castLowTide(t, e)

	for _, id := range []state.ObjID{warden, bear} {
		if got := e.G.Obj(id).Zone; got != state.ZGraveyard {
			t.Fatalf("creature %d zone = %s, want graveyard", id, got)
		}
	}
	if got := e.G.Obj(aura).Zone; got != state.ZExile {
		t.Fatalf("Aura zone = %s, want exile: nothing was left to enchant", got)
	}
}

// With a survivor it enchants the survivor, with no choice over creatures that
// were destroyed in the same event.
func TestCR610_3AuraReturnEnchantsTheSurvivor(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, testLowTide))
	warden := onBoard(t, e, 0, testWarden)
	onBoard(t, e, 1, testWarden)
	survivor := onBoard(t, e, 1, testColossus)
	aura := holdUntilHostLeaves(t, e, warden, testSnare)

	castLowTide(t, e)

	if o := e.G.Obj(aura); o.Zone != state.ZBattlefield || o.AttachedTo != survivor {
		t.Fatalf("Aura zone %s attached to %d, want the battlefield on the survivor %d", o.Zone, o.AttachedTo, survivor)
	}
}

// State-based destruction is one event too (CR 704.3).
func TestCR610_3AuraReturnFindsTheBoardAfterLethalDamage(t *testing.T) {
	t.Parallel()
	e := layerEngine(t)
	warden := onBoard(t, e, 0, testWarden)
	bear := onBoard(t, e, 1, testWarden)
	aura := holdUntilHostLeaves(t, e, warden, testSnare)
	for _, id := range []state.ObjID{warden, bear} {
		e.emit(events.Event{Kind: events.Damage, Obj: id, Amount: 2})
	}

	e.checkStateBased()

	if got := e.G.Obj(aura).Zone; got != state.ZExile {
		t.Fatalf("Aura zone = %s, want exile: nothing was left to enchant", got)
	}
}

// CR 610.3d: cards returning after one event return at the same time, so each
// sees the other enter (CR 603.6a).
func TestCR610_3dReturnsFromOneEventAreSimultaneous(t *testing.T) {
	t.Parallel()
	const watcher = "Name:Test Lookout\nManaCost:3\nTypes:Creature Human\nPT:3/3\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Creature.Other | TriggerZones$ Battlefield | Execute$ TestLookoutLife | TriggerDescription$ Another arrival earns you a life.\n" +
		"SVar:TestLookoutLife:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"
	e := handEngine(t, card(t, testLowTide))
	wardenA := onBoard(t, e, 0, testWarden)
	wardenB := onBoard(t, e, 0, testWarden)
	lookoutA := holdUntilHostLeaves(t, e, wardenA, watcher)
	lookoutB := holdUntilHostLeaves(t, e, wardenB, watcher)
	e.G.Players[0].Pool[state.MW] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	for i := 0; i < 2 && e.G.Obj(lookoutA).Zone != state.ZBattlefield; i++ {
		d := e.Pending()
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{passPriorityOption(t, d)}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	if e.G.Obj(lookoutA).Zone != state.ZBattlefield || e.G.Obj(lookoutB).Zone != state.ZBattlefield {
		t.Fatalf("zones %s and %s, want both back on the battlefield", e.G.Obj(lookoutA).Zone, e.G.Obj(lookoutB).Zone)
	}
	if got := len(e.G.Stack) + len(e.pendingTriggers); got != 2 {
		t.Fatalf("%d triggers, want 2: each returning creature sees the other", got)
	}
}

// A sacrifice over several players is one event: a card its host held stays
// exiled while the next player chooses, and comes back once every player has
// answered. The second answer re-executes the resolution from its checkpoint,
// which fails on anything logged after the posed ask.
func TestCR610_3ReturnWaitsForTheNextPlayersSacrifice(t *testing.T) {
	t.Parallel()
	const cull = "Name:Test Mutual Cull\nManaCost:W\nTypes:Sorcery\n" +
		"A:SP$ Sacrifice | Defined$ Player | SacValid$ Creature | Amount$ 2 | SpellDescription$ Everyone gives up a pair of creatures.\nOracle:x\n"
	e := handEngine(t, card(t, cull))
	warden := onBoard(t, e, 0, testWarden)
	for _, p := range []state.PlayerID{0, 0, 1, 1, 1} {
		onBoard(t, e, p, testWarden)
	}
	held := holdUntilHostLeaves(t, e, warden, testColossus)
	e.G.Players[0].Pool[state.MW] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")

	answered := 0
	for answered < 2 {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision before both players sacrificed")
		}
		if d.Kind == decision.KPriority {
			submitChoices(t, e, passPriorityOption(t, d))
			continue
		}
		if got := e.L.Events[len(e.L.Events)-1].Kind; got != events.DecisionAsk {
			t.Fatalf("last event of a posed sacrifice is %v, want its ask", got)
		}
		if got := e.G.Obj(held).Zone; got != state.ZExile {
			t.Fatalf("held card zone = %s while seat %d chooses, want exile", got, d.Player)
		}
		submitChoices(t, e, 0, 1)
		answered++
	}
	if got := e.G.Obj(held).Zone; got != state.ZBattlefield {
		t.Fatalf("held card zone = %s after every sacrifice, want the battlefield", got)
	}
}
