package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// An "exile until this leaves the battlefield" host (the Oblivion Ring /
// Banisher Priest pattern, Duration$ UntilHostLeavesPlay) whose controller
// DIES to lethal combat damage returns its exiled card in the state-based
// pass that follows the damage pass (CR 704.3b, 800.4a), and the returned
// card's as-enters choice (CR 610.3d) is posed to its owner, never answered
// with its deterministic default. The damage pass is a turn-based pass the
// resolution kernel may cover with a tape run, so the ask is served inside
// it — which is exactly what the pass gate (tapeStepMayAsk's combat arm
// counting the pending return) and the run's end (completeCombatPass, after
// the SBA sweep, not runCombatAssignments' tail) exist to make true.
//
// The fixture is inline (the licensing rule); the exiled card's script is the
// corpus Outpost Siege's own SVar shape (K:ETBReplacement:Other +
// DB$ GenericChoice), minimised to a two-mode choice whose bodies are stand-ins.
func TestExileReturnAsksAfterLethalCombatDamage(t *testing.T) {
	t.Parallel()
	host := card(t, "Name:Test Grasp\nManaCost:2 W\nTypes:Enchantment\n"+
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigExile\n"+
		"SVar:TrigExile:DB$ ChangeZone | Origin$ Battlefield | Destination$ Exile | ValidTgts$ Permanent.nonLand+OppCtrl | TgtPrompt$ Select target | Duration$ UntilHostLeavesPlay\n"+
		"Oracle:x\n")
	siege := "Name:Test Siege\nManaCost:3 R\nTypes:Enchantment\n" +
		"K:ETBReplacement:Other:PickSide\n" +
		"SVar:PickSide:DB$ GenericChoice | Defined$ You | SetChosenMode$ True | Choices$ Dawn,Wards\n" +
		"SVar:Dawn:DB$ Nothing\n" +
		"SVar:Wards:DB$ Nothing\n" +
		"Oracle:x\n"
	attacker := "Name:Test Bearer\nManaCost:3 R\nTypes:Creature Beast\nPT:5/5\nOracle:x\n"

	e := handEngine(t, host)
	siegeID := onBoard(t, e, 1, siege)
	bearer := onBoardReady(t, e, 1, attacker)
	e.G.Players[0].Life = 4

	// Precondition: seat 1 is alive and its bearer is on the battlefield,
	// because seat 1's lethal attack is what sweeps seat 0 (and its host) out.
	if e.G.Players[1].Lost {
		t.Fatal("precondition: seat 1 is already lost")
	}
	if o := e.G.Obj(bearer); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: seat 1's bearer = %+v", o)
	}

	// The host exiles the Siege (its ETB trigger poses the exile target ask).
	e.G.Players[0].Pool[state.MW] = 1
	e.G.Players[0].Pool[state.MC] = 2
	e.askPriority(0)
	castFirst(t, e, "cast")
	d := passToTargetAsk(t, e)
	idx := -1
	for _, o := range d.Options {
		if o.Obj == siegeID {
			idx = o.Index
		}
	}
	if idx < 0 {
		t.Fatalf("seat 1's Siege not offered as the exile target: %+v", d.Options)
	}
	submitChoices(t, e, idx)
	passUntilStackEmpty(t, e, 8)
	// Precondition the return rule reads: the Siege sits in exile under the
	// host's UntilHostLeavesPlay entry (compare the zones, not the pointer).
	if got := e.G.Obj(siegeID).Zone; got != state.ZExile {
		t.Fatalf("precondition: Siege zone = %v, want exile", got)
	}
	light := state.ObjID(0)
	for _, o := range e.G.Zone(state.ZBattlefield, 0) {
		light = o
	}
	if len(e.G.Obj(light).ExileReturn) != 1 || e.G.Obj(light).ExileReturn[0].Obj != siegeID ||
		e.G.Obj(light).ExileReturn[0].From != state.ZBattlefield {
		t.Fatalf("precondition: host ExileReturn = %+v, want one battlefield entry naming the Siege",
			e.G.Obj(light).ExileReturn)
	}

	// Seat 1 attacks seat 0 with the unblocked bearer: 5 damage to 4 life is
	// lethal, seat 0 leaves in the state-based pass, and the Siege returns.
	// Seat 0 has no creatures, so no blockers decision is ever posed; the
	// priority drain below crosses the declare-blockers window and the pass
	// into the combat damage step, which deals the damage.
	e.G.Step = state.StepDeclareAttackers
	e.G.Active, e.G.Priority = 1, 1
	e.askAttackers()
	submitAttackersOnly(t, e, bearer)
	drainCombatPriority(t, e)
	for i := 0; i < 32; i++ {
		d = e.Pending()
		if d == nil {
			t.Fatal("no pending decision after the lethal pass")
		}
		if d.Kind == decision.KModes {
			break
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("expected priority or the returned Siege's as-enters ask, got %+v", d)
		}
		idx = -1
		for _, o := range d.Options {
			if o.Kind == "pass" {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("priority decision with no pass option: %+v", d)
		}
		if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: []int{idx}}); err != nil {
			t.Fatalf("submit pass: %v", err)
		}
	}
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("the returned Siege's as-enters choice was never posed; last pending = %+v", d)
	}
	if d.Player != 1 {
		t.Fatalf("as-enters choice posed to seat %d, want the returned Siege's owner (seat 1)", d.Player)
	}
	if len(d.Options) != 2 {
		t.Fatalf("mode options = %d, want the two GenericChoice candidates", len(d.Options))
	}
	if hasNote(e, "no resolution run serves it") {
		t.Fatal("the as-enters choice was answered with its default: no run served it")
	}
	// The answer is a real one: choose mode 0, then the game must carry the
	// chosen mode on the returned Siege.
	submitChoices(t, e, 0)
	if got := e.G.Obj(siegeID).Zone; got != state.ZBattlefield {
		t.Fatalf("returned Siege zone = %v, want battlefield", got)
	}
	passUntilStackEmpty(t, e, 8)
	if !e.G.Players[0].Lost {
		t.Fatal("seat 0 should have lost to the lethal combat damage")
	}
	if len(e.G.Obj(siegeID).ChosenModes) == 0 {
		t.Fatalf("the answered choice left no chosen mode on the Siege: %+v", e.G.Obj(siegeID).ChosenModes)
	}
}
