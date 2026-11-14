package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// spiritTokens counts the 1/1 white and black Spirit tokens seat 0 controls.
func spiritTokens(e *Engine) int {
	n := 0
	for _, id := range e.G.Zone(state.ZBattlefield, 0) {
		if o := e.G.Obj(id); o != nil && o.IsToken && o.Face() != nil && o.Face().Name == "Spirit Token" {
			n++
		}
	}
	return n
}

// TestGrantedAfterlifeMintsSpirits: a PumpAll "KW$ Afterlife:2" (Afterlife
// Insurance's shape, CR 702.135a) gives each creature its controller controls
// afterlife 2 until end of turn. The grant is a registered continuous effect
// whose AddKeywords carries Afterlife; a creature that dies under it must queue
// the printed expansion's look-back trigger (CR 603.10a) exactly once and make
// two Spirits. A creature that dies without the grant makes none, and the log
// replays to the identical game.
func TestGrantedAfterlifeMintsSpirits(t *testing.T) {
	e := handEngineTokens(t)
	src := onBoard(t, e, 0, "Name:Insurance\nManaCost:2 W\nTypes:Instant\nA:SP$ PumpAll | ValidCards$ Creature.YouCtrl | KW$ Afterlife:2 | SpellDescription$ x\nOracle:x\n")
	bear := onBoard(t, e, 0, "Name:Grizzly Bears\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	other := onBoard(t, e, 0, "Name:Runeclaw Bear\nManaCost:1 G\nTypes:Creature\nPT:2/2\nOracle:x\n")
	if len(e.G.Obj(bear).Face().Triggers) != 0 {
		t.Fatalf("precondition: the bears print %d triggers", len(e.G.Obj(bear).Face().Triggers))
	}
	if n := spiritTokens(e); n != 0 {
		t.Fatalf("precondition: %d Spirits before the pump", n)
	}
	// Control: an ungranted bear dying makes no Spirit.
	killCreature(t, e, other, 2)
	if e.G.Obj(other).Zone != state.ZGraveyard {
		t.Fatalf("precondition: the control bear is in %s, want the graveyard", e.G.Obj(other).Zone)
	}
	if n := spiritTokens(e); n != 0 {
		t.Fatalf("an ungranted creature's death made %d Spirits", n)
	}

	f := e.G.Obj(src).Face()
	e.resolveAbility(src, 0, nil, f.SpellAbility(), f.SVars)
	if !e.HasKeyword(bear, "Afterlife") {
		t.Fatalf("precondition: the pumped bear keywords %v lack Afterlife", e.Derived(bear).Keywords)
	}
	killCreature(t, e, bear, 2)
	if len(e.G.Stack) != 1 {
		t.Fatalf("precondition: stack %v after the death, want the one granted trigger", e.G.Stack)
	}
	e.priorityRound()
	drainStackEmpty(t, e, 20)
	if e.G.Obj(bear).Zone != state.ZGraveyard {
		t.Fatalf("precondition: the bear is in %s, want the graveyard", e.G.Obj(bear).Zone)
	}
	if n := spiritTokens(e); n != 2 {
		t.Fatalf("a creature granted afterlife 2 died and left %d Spirits, want 2", n)
	}
	pushes := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.GrantTriggerPush {
			pushes++
		}
	}
	if pushes != 1 {
		t.Fatalf("granted Afterlife pushed %d triggers, want 1 (read once, in the look-back pass)", pushes)
	}
}
