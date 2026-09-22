package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/cards"
	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/events"
	"github.com/adams-shaun/gorge/state"
)

func TestWitherDamageToCreatureIsMinusOneCountersNotMarkedDamage(t *testing.T) {
	e, _, source := newFixtureDeck(t, 2, "Name:Wither Source\nTypes:Creature\nPT:2/2\nK:Wither\nOracle:x\n")
	target := onBoard(t, e, 1, "Name:Target\nTypes:Creature\nPT:3/3\nOracle:x\n")
	if !e.HasKeyword(source, "Wither") {
		t.Fatal("precondition: source does not read printed Wither")
	}
	if e.G.Obj(target).Zone != state.ZBattlefield || !e.IsCreature(target) {
		t.Fatal("precondition: target is not a battlefield creature")
	}
	effects.Resolve(e, &effects.Ctx{Source: source, Controller: 0,
		Targets: []state.Target{{Obj: target}}}, &cards.SA{Kind: "DB", API: "DealDamage",
		Params: map[string]string{"Defined": "Targeted", "NumDmg": "2"}})
	if got := e.G.Obj(target).Counter("M1M1"); got != 2 {
		t.Fatalf("target -1/-1 counters = %d, want 2", got)
	}
	if got := e.G.Obj(target).Damage; got != 0 {
		t.Fatalf("target marked damage = %d, want 0", got)
	}
	count := 0
	for _, ev := range e.L.Events {
		if ev.Kind == events.CounterChange && ev.Obj == target && ev.Counter == "M1M1" {
			count++
			if ev.Amount != 2 {
				t.Fatalf("counter placement amount = %d, want 2", ev.Amount)
			}
		}
	}
	if count != 1 {
		t.Fatalf("M1M1 CounterChange count = %d, want 1", count)
	}
}
