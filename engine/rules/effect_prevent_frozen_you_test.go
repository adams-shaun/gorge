package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// A temporary "prevent all damage that would be dealt to you" replacement
// means the player who resolved the spell that made it (CR 109.5, 608.2), not
// whoever owns or controls the card that made it afterwards. Eerie
// Interference cast from an opponent's exile is the real shape: the card's
// owner is seat 0, the caster seat 1.
func TestEffectPreventionYouIsTheResolvingController(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	const fixture = "Name:Shield Source Fixture\nTypes:Creature\nPT:1/1\nOracle:x\n"
	e, _ := combatTriggerBoard(t, reg, nil, []string{fixture}, nil, []string{damagePreventedOnceAggressor})
	src := findBattlefield(t, e, 0, "Shield Source Fixture", 0)
	hitter := findBattlefield(t, e, 1, "Aggressor", 0)
	e.AddContinuous(state.ContinuousEffect{Source: src, Controller: 1, UntilEOT: true,
		ReplacementEvent: "DamageDone",
		ReplacementParams: map[string]string{"Prevent": "True",
			"ValidSource": "Creature.inZoneBattlefield", "ValidTarget": "You,Creature.YouCtrl"}})
	life0, life1 := e.G.Players[0].Life, e.G.Players[1].Life
	e.damaging, e.combatDamaging = hitter, false
	e.emit(events.Event{Kind: events.Damage, Player: 0, Amount: 3})
	e.emit(events.Event{Kind: events.Damage, Player: 1, Amount: 3})
	e.damaging = 0
	if got := e.G.Players[0].Life; got != life0-3 {
		t.Fatalf("seat 0 (the card's controller, not the caster) life = %d, want %d", got, life0-3)
	}
	if got := e.G.Players[1].Life; got != life1 {
		t.Fatalf("seat 1 (the caster) life = %d, want %d: its damage was not prevented", got, life1)
	}
}
