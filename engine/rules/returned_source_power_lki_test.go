package rules

// CR 400.7 / 608.2h: a permanent that leaves the battlefield and comes back is
// a new object, so an ability its old self put on the stack reads that old
// object's last known information, not the returned one. A 1/2 with undying
// activates "this deals damage equal to its power to each opponent"; it dies
// in response and undying returns it as a 2/3. The ability deals 1 (Gleeful
// Arsonist's trigger after Kaervek the Merciless kills it; the kernel already
// deals 1).

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const returnedSparkSrc = "Name:Test Returning Spark\nManaCost:1 R\nTypes:Creature Human Wizard\nPT:1/2\n" +
	"K:Undying\n" +
	"A:AB$ DealDamage | NumDmg$ SparkX | Defined$ Player.Opponent | Cost$ 1\n" +
	"SVar:SparkX:Count$CardPower\n" +
	"Oracle:x\n"

func TestReturnedSourcePowerIsTheOldObjectsLastKnown(t *testing.T) {
	t.Parallel()
	e, cfg, find := etbConfig(t, 115, []string{returnedSparkSrc}, nil)
	spark := find("Test Returning Spark", 0)
	e.emit(events.Event{Kind: events.MoveZone, Obj: spark, From: state.ZHand, To: state.ZBattlefield})
	inc := e.G.Obj(spark).Incarnation
	addMana(t, e, 0, "R")
	submitChoices(t, e, abilityOption(t, e, spark, 0).Index)
	if len(e.G.Stack) != 1 || e.G.Obj(e.G.Stack[0]).Source != spark {
		t.Fatalf("precondition: stack %v, want the spark's ability waiting", e.G.Stack)
	}
	life1 := e.G.Players[1].Life
	e.emit(events.Event{Kind: events.MoveZone, Obj: spark, From: state.ZBattlefield, To: state.ZGraveyard, Text: "destroyed"})
	passUntilStackEmpty(t, e, 40)
	o := e.G.Obj(spark)
	if o.Zone != state.ZBattlefield || o.Incarnation == inc || e.Power(spark) != 2 {
		t.Fatalf("precondition: spark zone %v incarnation %d (was %d) power %d, want it returned by undying as a new 2/3",
			o.Zone, o.Incarnation, inc, e.Power(spark))
	}
	if got := life1 - e.G.Players[1].Life; got != 1 {
		t.Errorf("opponent lost %d life, want 1: the ability reads the departed 1/2's power, not the returned 2/3's (CR 400.7, 608.2h)", got)
	}
	replayCheck(t, e, cfg)
}
