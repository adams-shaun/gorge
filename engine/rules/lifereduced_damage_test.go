package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// Bloodletter of Aclazotz's LifeReduced replacement doubles the life LOSS
// caused by damage, not the damage itself (CR 120.3a/119.3). Lifelink and
// damage-dealt triggers must still see the damage dealt.
const lifelinkDamageTriggerSrc = "Name:Cleric\nManaCost:2 W\nTypes:Creature Cleric\nPT:3/3\nK:Lifelink\n" +
	"T:Mode$ DamageDealtOnce | CombatDamage$ True | ValidSource$ Card.Self | Execute$ TrigCounter | TriggerZones$ Battlefield | TriggerDescription$ x\n" +
	"SVar:TrigCounter:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ X\n" +
	"SVar:X:TriggerCount$DamageAmount\nOracle:x\n"

func TestLifeReducedTwiceKeepsDamageDealt(t *testing.T) {
	t.Run("combat damage", func(t *testing.T) {
		e := combatEngine(t)
		onBoardCard(t, e, 0, corpusCard(t, "Bloodletter of Aclazotz"))
		atk := onBoardReady(t, e, 0, lifelinkDamageTriggerSrc)

		e.askAttackers()
		submitAttackers(t, e, atk)
		drainCombatDamagePriority(t, e)
		passUntilStackEmpty(t, e, 30)

		if got := e.G.Players[1].Life; got != 14 {
			t.Fatalf("defender life = %d, want 14 (3 damage, doubled to 6 life loss)", got)
		}
		if got := e.G.Players[0].Life; got != 23 {
			t.Fatalf("attacker controller life = %d, want 23 (20 + 3 lifelink, not 6)", got)
		}
		if got := e.G.Players[1].LifeLostThisTurn; got != 6 {
			t.Fatalf("defender life lost this turn = %d, want 6", got)
		}
		if got := e.G.Obj(atk).Counter("P1P1"); got != 3 {
			t.Fatalf("damage trigger counters = %d, want 3 (the damage dealt, not the doubled loss)", got)
		}
		found := false
		for _, ev := range e.L.Events {
			if ev.Kind == events.Damage && ev.Player == 1 {
				if ev.Amount != 3 {
					t.Fatalf("Damage amount = %d, want 3 (damage dealt is unchanged)", ev.Amount)
				}
				if loss, ok := events.DamageLifeLoss(ev); !ok || loss != 6 {
					t.Fatalf("recorded life loss = %d (ok %v), want 6", loss, ok)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("no Damage event against the defender")
		}
	})

	t.Run("noncombat damage", func(t *testing.T) {
		reg := testutil.CorpusRegistry(t)
		e, cfg := linkBoard(t, reg,
			[]string{"Bloodletter of Aclazotz", "Brion Stoutarm", "Grizzly Bears"}, nil)
		var brion, fodder state.ObjID
		for i := range e.G.Objs {
			o := &e.G.Objs[i]
			if o.Zone != state.ZBattlefield {
				continue
			}
			switch o.Face().Name {
			case "Brion Stoutarm":
				brion = o.ID
			case "Grizzly Bears":
				fodder = o.ID
			}
		}
		if brion == 0 || fodder == 0 {
			t.Fatalf("board missing Brion or fodder: %d %d", brion, fodder)
		}
		addMana(t, e, 0, "R")
		opt := abilityOption(t, e, brion, 0)
		submitChoices(t, e, opt.Index)
		activateAnswers(t, e, fodder, 0, 1)
		passUntilStackEmpty(t, e, 30)

		if got := e.G.Players[1].Life; got != 16 {
			t.Fatalf("defender life = %d, want 16 (2 damage, doubled to 4 life loss)", got)
		}
		if got := e.G.Players[0].Life; got != 22 {
			t.Fatalf("Brion's controller life = %d, want 22 (20 + 2 lifelink, not 4)", got)
		}
		found := false
		for _, ev := range e.L.Events {
			if ev.Kind == events.Damage && ev.Player == 1 {
				if ev.Amount != 2 {
					t.Fatalf("Damage amount = %d, want 2 (damage dealt is unchanged)", ev.Amount)
				}
				if loss, ok := events.DamageLifeLoss(ev); !ok || loss != 4 {
					t.Fatalf("recorded life loss = %d (ok %v), want 4", loss, ok)
				}
				found = true
			}
		}
		if !found {
			t.Fatal("no Damage event against the defender")
		}
		replayCheck(t, e, cfg)
	})
}
