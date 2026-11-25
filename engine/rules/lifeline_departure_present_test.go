package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

// The CR 603.4 intervening-if on a leaves-the-battlefield trigger is checked
// against the game state WHEN the event occurs (Forge and XMage both read the
// post-departure board): the departing creature is no longer on the
// battlefield, so it cannot satisfy its own "another creature" clause.
// Lifeline ("Whenever a creature dies, if another creature is on the
// battlefield, ...") must therefore not trigger when the ONLY creature dies,
// and must trigger when a second creature remains. The fixtures are
// inline-authored (never corpus .txt, per the licensing rule) and mirror the
// Forge scripts' shapes.
var llLifeline = "Name:Lifeline Fixture\nManaCost:5\nTypes:Artifact\n" +
	"T:Mode$ ChangesZone | ValidCard$ Card.Creature | Origin$ Battlefield | Destination$ Graveyard | IsPresent$ Creature | Execute$ TrigLLDelay | TriggerZones$ Battlefield | TriggerDescription$ Whenever a creature dies, if another creature is on the battlefield, return the first card to the battlefield at the beginning of the next end step.\n" +
	"SVar:TrigLLDelay:DB$ DelayedTrigger | Mode$ Phase | Phase$ End of Turn | ValidPlayer$ Player | Execute$ TrigLLReturn | RememberObjects$ TriggeredNewCardLKICopy\n" +
	"SVar:TrigLLReturn:DB$ ChangeZone | Defined$ DelayTriggerRememberedLKI | Origin$ Graveyard | Destination$ Battlefield\n" +
	"Oracle:x\n"

var llKami = "Name:Kami Fixture\nManaCost:W\nTypes:Creature Spirit\nPT:1/1\n" +
	"A:AB$ Fog | Cost$ Sac<1/CARDNAME> | SpellDescription$ Prevent all combat damage that would be dealt this turn.\n" +
	"Oracle:x\n"

var llBear = "Name:Bear Fixture\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// llSacrificeKami activates the Kami fixture's own sacrifice-cost ability
// (the cost payment emits the death event) and hands back to the caller,
// which must drain it through llFired: a trigger queued by the death sits in
// pendingTriggers until the next priority round, so a plain
// passUntilStackEmpty could finish past it unseen.
func llSacrificeKami(t *testing.T, e *Engine, kami state.ObjID) {
	t.Helper()
	e.priorityRound()
	d := e.Pending()
	if d == nil {
		t.Fatal("no priority decision for the Kami activation")
	}
	for _, o := range d.Options {
		if o.Kind == "ability" && o.Obj == kami {
			submitChoices(t, e, o.Index)
			return
		}
	}
	t.Fatalf("no ability option for %d: %+v", kami, d.Options)
}

// llFired reports whether Lifeline's trigger was ever QUEUED (pendingTriggers)
// or placed on the stack while the drain passes priority -- a trigger that
// fires and resolves inside one drain is off the stack by the end of it, so
// the watcher must look at every step. The drain answers each pass the way a
// player would (pass), and a mid-drain ordering/choose ask is answered with
// its first option.
func llFired(t *testing.T, e *Engine, lifeline state.ObjID) bool {
	t.Helper()
	for n := 0; n < 30; n++ {
		for _, pt := range e.pendingTriggers {
			if pt.Source == lifeline {
				return true
			}
		}
		for _, id := range e.G.Stack {
			if o := e.G.Obj(id); o != nil && o.Source == lifeline {
				return true
			}
		}
		if len(e.G.Stack) == 0 && len(e.pendingTriggers) == 0 {
			return false
		}
		passOnce(t, e)
	}
	t.Fatal("stack never emptied within the watcher budget")
	return false
}

// The lone creature dies as an ability cost: with nothing else on the
// battlefield, Lifeline's intervening-if ("if another creature is on the
// battlefield") is false at the moment the death occurs, so it must not
// trigger -- the dying creature is already off the battlefield and cannot be
// the "another creature" its own death is measured against.
func TestLifelineNoTriggerWhenOnlyCreatureDies(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 1411, llLifeline, llKami)
	lifeline := putCreature(t, e, 0, llLifeline)
	kami := putCreature(t, e, 0, llKami)
	if e.G.Obj(lifeline) == nil || e.G.Obj(kami) == nil {
		t.Fatal("precondition: Lifeline and Kami on the battlefield required")
	}
	if len(e.G.Zone(state.ZBattlefield, 0)) != 2 {
		t.Fatalf("precondition: battlefield = %v, want exactly the two cards", e.G.Zone(state.ZBattlefield, 0))
	}

	llSacrificeKami(t, e, kami)

	if o := e.G.Obj(kami); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: sacrificed Kami zone = %v, want graveyard", o)
	}
	if llFired(t, e, lifeline) {
		t.Fatalf("Lifeline triggered for the only creature's own death: stack = %v", e.G.Stack)
	}
	if len(e.G.Delayed) != 0 {
		t.Fatalf("Lifeline's delayed return registered despite the lone death: %+v", e.G.Delayed)
	}
}

// With a second creature present, the same death satisfies the intervening-if
// and Lifeline triggers, registering its end-of-turn return.
func TestLifelineTriggersWithSecondCreature(t *testing.T) {
	t.Parallel()
	e, _, _ := newFixtureDeck(t, 1412, llLifeline, llKami, llBear)
	lifeline := putCreature(t, e, 0, llLifeline)
	kami := putCreature(t, e, 0, llKami)
	bear := putCreature(t, e, 0, llBear)
	if e.G.Obj(lifeline) == nil || e.G.Obj(kami) == nil || e.G.Obj(bear) == nil {
		t.Fatal("precondition: Lifeline, Kami and Bear on the battlefield required")
	}

	llSacrificeKami(t, e, kami)

	if o := e.G.Obj(kami); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: sacrificed Kami zone = %v, want graveyard", o)
	}
	if !llFired(t, e, lifeline) {
		t.Fatalf("Lifeline did not trigger with a second creature present: stack = %v", e.G.Stack)
	}
}
