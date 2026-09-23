package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

// The DamageDone trigger mode is the Azra Oddsmaker Effect shape. The
// deliberately simple Execute body makes each separate firing observable.
func TestEffectDamageDoneTriggerRepeatsWithinTurn(t *testing.T) {
	promise := card(t, "Name:OddsmakerPromise\nManaCost:U\nTypes:Sorcery\n"+
		"A:SP$ Effect | Triggers$ TrigDamage\n"+
		"SVar:TrigDamage:Mode$ DamageDone | ValidTarget$ Player | TriggerZones$ Command | Execute$ TrigPain\n"+
		"SVar:TrigPain:DB$ LoseLife | Defined$ You | LifeAmount$ 2\nOracle:x\n")
	e := handEngine(t, promise)
	e.G.Players[0].Pool[state.MU] = 1
	e.askPriority(0)
	castFirst(t, e, "cast")
	passUntilStackEmpty(t, e, 8)
	if src := e.G.Obj(e.G.Zone(state.ZGraveyard, 0)[0]); src == nil || src.Face().Name != "OddsmakerPromise" {
		t.Fatalf("precondition: Effect spell did not reach the graveyard: %+v", src)
	}
	if len(e.G.Delayed) != 1 || e.G.Delayed[0].EventMode != "DamageDone" || !e.G.Delayed[0].EffectRepeat {
		t.Fatalf("precondition: Effect did not arm a repeatable DamageDone trigger: %+v", e.G.Delayed)
	}
	before := e.G.Players[0].Life
	// Direct noncombat damage to an opponent exercises ValidTarget$ Player;
	// the triggered body must lose life for the Effect's controller twice.
	for n := 1; n <= 2; n++ {
		e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 1})
		e.putTriggersOnStack()
		passUntilStackEmpty(t, e, 8)
		if got := e.G.Players[0].Life; got != before-int32(2*n) {
			t.Fatalf("after %d hits, controller life = %d, want %d", n, got, before-int32(2*n))
		}
	}
}
