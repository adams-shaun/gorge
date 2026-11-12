package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/effects"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Decorated Griffin's shield is "the next 1 combat damage that would be dealt to
// you this turn" (CombatDamage$ True): noncombat damage does not touch it and
// does not spend it; the first combat damage does.
func TestCombatOnlyPreventionShield(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e := newSeats(t, 2)
	griffin := onBoardCard(t, e, 0, mustCorpusCard(t, reg, "Decorated Griffin"))
	sa := e.G.Obj(griffin).Face().Abilities[0]
	effects.Resolve(e, &effects.Ctx{Source: griffin, Controller: 0}, sa)
	if got := countShieldCEs(e); got != 1 {
		t.Fatalf("shields = %d, want 1", got)
	}
	life := e.G.Players[0].Life
	e.damaging = griffin
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 1})
	e.damaging = 0
	if got := e.G.Players[0].Life; got != life-1 {
		t.Fatalf("noncombat damage: life = %d, want %d (a combat-only shield prevents none)", got, life-1)
	}
	if got := countShieldCEs(e); got != 1 {
		t.Fatalf("shields = %d, want 1 after noncombat damage", got)
	}
	e.damaging, e.combatDamaging = griffin, true
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 1})
	e.damaging, e.combatDamaging = 0, false
	if got := e.G.Players[0].Life; got != life-1 {
		t.Fatalf("combat damage: life = %d, want %d (prevented)", got, life-1)
	}
	if got := countShieldCEs(e); got != 0 {
		t.Fatalf("shields = %d, want 0 (spent)", got)
	}
}
