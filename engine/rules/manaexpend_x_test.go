package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// An expend threshold no test reaches: the watcher is here only so that a
// cast emits the expend wake-up event.
const expendWatcherSrc = "Name:Expend Watcher\nManaCost:1\nTypes:Creature Bird\nPT:1/1\n" +
	"T:Mode$ ManaExpend | Amount$ 50 | Player$ You | TriggerZones$ Battlefield | Execute$ WatcherGain | TriggerDescription$ Fixture watcher.\n" +
	"SVar:WatcherGain:DB$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"

// assertExpendWakeEmitted guards against a vacuous pass: the X assertions mean
// nothing unless the cast really emitted a wake-up event spending want mana.
func assertExpendWakeEmitted(t *testing.T, e *Engine, want int32) {
	t.Helper()
	for i := range e.L.Events {
		ev := &e.L.Events[i]
		if ev.Kind == events.CastInfo && events.FlagsFrom(ev.Counter)&state.FlagManaExpendCast != 0 {
			if ev.Amount != want {
				t.Fatalf("expend wake-up Amount = %d, want %d", ev.Amount, want)
			}
			return
		}
	}
	t.Fatal("the cast emitted no expend wake-up event")
}

// CR 107.3: X is the announced value, so a spell with no {X} in its cost keeps
// X 0 whatever its payment spent.
func TestExpendWakeLeavesNonXSpellXZero(t *testing.T) {
	t.Parallel()
	src := "Name:Plain Expend Spell\nManaCost:2 W\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ 1\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 211, src, expendWatcherSrc)
	putCreature(t, e, 0, expendWatcherSrc)
	addMana(t, e, 0, "WWW")
	submitChoices(t, e, castOptionFor(t, e, id).Index)

	assertExpendWakeEmitted(t, e, 3)
	if got := e.G.Obj(id).X; got != 0 {
		t.Fatalf("X = %d on a spell with no {X}, want 0", got)
	}
	replayCheck(t, e, cfg)
}

// CR 107.3 and CR 202.3e: the announced X, not the total mana spent, is what
// the spell's mana value on the stack and its resolution read.
func TestExpendWakeKeepsAnnouncedX(t *testing.T) {
	t.Parallel()
	src := "Name:Scaling Expend Spell\nManaCost:X W\nTypes:Sorcery\nA:SP$ GainLife | Defined$ You | LifeAmount$ X\nSVar:X:Count$xPaid\nOracle:x\n"
	e, cfg, id := newFixtureDeck(t, 212, src, expendWatcherSrc)
	putCreature(t, e, 0, expendWatcherSrc)
	addMana(t, e, 0, "WWW")
	submitChoices(t, e, castOptionFor(t, e, id).Index)
	chooseX(t, e, 2)

	assertExpendWakeEmitted(t, e, 3)
	o := e.G.Obj(id)
	if o.X != 2 {
		t.Fatalf("X = %d, want the announced 2", o.X)
	}
	if got := state.ManaValue(e.G, o); got != 3 {
		t.Fatalf("mana value on the stack = %d, want 3", got)
	}
	life := e.G.Players[0].Life
	passUntilStackEmpty(t, e, 20)
	if got := e.G.Players[0].Life - life; got != 2 {
		t.Fatalf("gained %d life, want 2", got)
	}
	replayCheck(t, e, cfg)
}
