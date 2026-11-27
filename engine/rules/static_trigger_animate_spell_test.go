package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// Thundermane Dragon's static SpellCast rider: "If you cast a creature spell
// this way, it gains haste until end of turn." The trigger resolves off the
// stack (the ask-free static shape), and the Animate it runs is a timed keyword
// grant on the triggering spell. CR 400.7a carries a spell-on-the-stack grant
// onto the permanent the spell becomes, so the creature must enter with haste
// and keep it only until the end of the turn.
const staticTriggerAnimateWatcher = "Name:Thundermane Watcher\nTypes:Creature Dragon\nPT:4/4\n" +
	"T:Mode$ SpellCast | ValidCard$ Creature | ValidActivatingPlayer$ You | TriggerZones$ Battlefield | Static$ True | Execute$ Body\n" +
	"SVar:Body:DB$ Animate | Defined$ TriggeredCardLKICopy | Keywords$ Haste\n" +
	"Oracle:x\n"

const staticTriggerAnimateBears = "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

func TestStaticTriggerAnimateSpellKeepsHaste(t *testing.T) {
	t.Parallel()
	e := handEngine(t, card(t, staticTriggerAnimateBears))
	onBoard(t, e, 0, staticTriggerAnimateWatcher)
	id := e.G.Zone(state.ZHand, 0)[0]
	e.G.Players[0].Pool[state.MG] = 2
	e.priorityRound()

	submitChoices(t, e, passToCast(t, e, id))
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack = %v, want the Grizzly Bears spell", e.G.Stack)
	}
	spell := e.G.Stack[0]
	if !e.HasKeyword(spell, "Haste") {
		t.Fatalf("precondition: the static SpellCast rider did not grant the spell haste (derived %v)", e.Derived(spell).Keywords)
	}

	passUntilStackEmpty(t, e, 40)
	if o := e.G.Obj(id); o == nil || o.Zone != state.ZBattlefield {
		t.Fatalf("precondition: Grizzly Bears = %+v, want on the battlefield", e.G.Obj(id))
	}
	if !e.HasKeyword(id, "Haste") {
		t.Fatalf("Grizzly Bears entered without the rider's haste (derived %v)", e.Derived(id).Keywords)
	}

	akBeginNextTurn(t, e)
	if e.HasKeyword(id, "Haste") {
		t.Fatalf("the until-end-of-turn haste survived the turn boundary (derived %v)", e.Derived(id).Keywords)
	}
}
