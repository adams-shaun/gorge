package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The corpus Dr. McCoy, Surly Surgeon: a Murder on another creature of its
// controller's that carries a +1/+1 counter removes the counters and heals the
// damage instead (the HealDamage carrier the corpus census names).
func TestCorpusMcCoySparesACounteredCreatureFromDestroy(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	mccoy := mustCorpusCard(t, reg, "Dr. McCoy, Surly Surgeon")
	murder := mustCorpusCard(t, reg, "Murder")
	e, cfg := tokenReplGame(t, 7, mccoy, murder)
	spell := moveSeededCard(t, e, 0, murder, state.ZHand)
	bear := putToken(t, e, 0, destroyLineBearSrc, state.ZBattlefield)
	medic := moveSeededCard(t, e, 0, mccoy, state.ZBattlefield)
	// McCoy's own enters trigger asks for its +1/+1 counter's target once
	// priority is asked again.
	addMana(t, e, 0, "BBB")
	if d := e.Pending(); d != nil && d.Kind == decision.KTarget {
		submitChoices(t, e, indexOfObjOption(d, bear))
	}
	passUntilStackEmpty(t, e, 20)
	addMana(t, e, 0, "")
	e.emit(events.Event{Kind: events.CounterChange, Obj: bear, Counter: "P1P1", Amount: 2})
	e.emit(events.Event{Kind: events.Damage, Obj: bear, Amount: 1})
	addMana(t, e, 0, "")

	castDestroyAt(t, e, spell, bear)

	b := e.G.Obj(bear)
	if b.Zone != state.ZBattlefield || b.Counter("P1P1") != 0 || b.Damage != 0 {
		t.Fatalf("bear zone %s, %d counters, %d damage; want spared with none of either", b.Zone, b.Counter("P1P1"), b.Damage)
	}
	// The spared creature is another one; McCoy itself stays in play.
	if e.G.Obj(medic).Zone != state.ZBattlefield {
		t.Fatal("precondition: McCoy left the battlefield")
	}
	replayCheck(t, e, cfg)
}
