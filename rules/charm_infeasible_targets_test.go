package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/decision"
)

func TestDistinctCharmModeWithoutLegalTargetDoesNotFallback(t *testing.T) {
	charm := "Name:Infeasible Charm\nManaCost:B\nTypes:Instant\n" +
		"A:SP$ Charm | CharmNum$ 2 | Choices$ PlayerMode,CreatureMode\n" +
		"SVar:PlayerMode:DB$ LoseLife | ValidTgts$ Player | LifeAmount$ 2\n" +
		"SVar:CreatureMode:DB$ Destroy | ValidTgts$ Creature\n" +
		"Oracle:x\n"
	e, _, id := newFixtureDeck(t, 6311, charm)
	addMana(t, e, 0, "B")
	d := castFixture(t, e, id, -1)
	for i := 0; d != nil && d.Kind != decision.KModes && i < 10; i++ {
		submitChoices(t, e, 0)
		d = e.Pending()
	}
	if d == nil || d.Kind != decision.KModes {
		t.Fatalf("pending = %+v, want mode announcement", d)
	}
	submitChoices(t, e, 0, 1)
	d = e.Pending()
	if d == nil || d.Kind != decision.KTarget {
		t.Fatalf("pending = %+v, want mandatory combined target declaration", d)
	}
	if d.Min != 2 || d.Max != 2 {
		t.Fatalf("target bounds = %d..%d, want 2..2", d.Min, d.Max)
	}
	playerTargets, creatureTargets := 0, 0
	for _, opt := range d.Options {
		if opt.Group == "charm-mode-0" {
			playerTargets++
		}
		if opt.Group == "charm-mode-1" {
			creatureTargets++
		}
	}
	if playerTargets == 0 || creatureTargets != 0 {
		t.Fatalf("fixture legal target groups player=%d creature=%d options=%+v", playerTargets, creatureTargets, d.Options)
	}
	if e.G.Obj(id).Zone.String() != "stack" {
		t.Fatalf("spell zone=%s while mandatory declaration is pending", e.G.Obj(id).Zone)
	}
}
