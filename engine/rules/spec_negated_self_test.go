package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// negSelfWatcher is a creature with a dies trigger whose ValidCard$ carries
// the filter term `+<excl>`; the negated dotted spelling `!Card.Self` must
// read as "other than the source", exactly like `Other`.
func negSelfWatcher(valid string) string {
	return "Name:Neg Self Watcher\nManaCost:1 B\nTypes:Creature Zombie\nPT:2/2\n" +
		"T:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Graveyard | ValidCard$ " + valid +
		" | TriggerZones$ Battlefield | Execute$ TrigCtr | TriggerDescription$ x\n" +
		"SVar:TrigCtr:DB$ GainLife | LifeAmount$ 1\nOracle:x\n"
}

const negSelfBear = "Name:Neg Self Bear\nManaCost:1 G\nTypes:Creature Bear\nPT:2/2\nOracle:x\n"

// negSelfDeathQueues kills victim (the watcher itself when victimIsWatcher)
// and reports whether the watcher's dies trigger reached the stack.
func negSelfDeathQueues(t *testing.T, valid string, victimIsWatcher bool) bool {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
	e := corpusEngine(t, reg, nil, nil)
	w := onBoard(t, e, 0, negSelfWatcher(valid))
	bear := onBoard(t, e, 0, negSelfBear)
	victim := bear
	if victimIsWatcher {
		victim = w
	}
	e.emit(events.Event{Kind: events.Damage, Obj: victim, Amount: 2})
	e.checkStateBased()
	if o := e.G.Obj(victim); o == nil || o.Zone != state.ZGraveyard {
		t.Fatalf("precondition: victim must have died (valid %q)", valid)
	}
	if !victimIsWatcher {
		if o := e.G.Obj(w); o.Zone != state.ZBattlefield {
			t.Fatalf("precondition: watcher must stay on the battlefield, in %s", o.Zone)
		}
	}
	e.priorityRound()
	return len(e.G.Stack) == 1
}

// CR 109.5 / 603.10a: `Creature.YouOwn+!Card.Self` is "another creature you
// own": a bear's death triggers the watcher, the watcher's own death does
// not. The plain `Creature.YouOwn` control shows the watcher's own death is
// otherwise seen by look-back, so the negated term is what excludes it.
func TestSpecNegatedCardSelfExcludesOnlyTheSource(t *testing.T) {
	if !negSelfDeathQueues(t, "Creature.YouOwn", true) {
		t.Fatal("precondition: Creature.YouOwn must see the watcher's own death")
	}
	if !negSelfDeathQueues(t, "Creature.YouOwn", false) {
		t.Fatal("precondition: Creature.YouOwn must see the bear's death")
	}
	if !negSelfDeathQueues(t, "Creature.YouOwn+!Card.Self", false) {
		t.Error("Creature.YouOwn+!Card.Self: the bear's death did not trigger the watcher")
	}
	if negSelfDeathQueues(t, "Creature.YouOwn+!Card.Self", true) {
		t.Error("Creature.YouOwn+!Card.Self: the watcher's own death triggered it")
	}
}
