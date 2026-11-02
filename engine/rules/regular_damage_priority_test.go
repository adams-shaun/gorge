package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// CR 510.3 / 117.3a: after the regular combat damage is dealt the active
// player receives priority IN the combat damage step, so a trigger that
// damage put on the stack resolves there, before the end of combat step
// begins and before any "at end of combat" trigger (CR 511.2) exists. The
// regular pass used to move straight to end of combat, so the damage
// trigger resolved there, after the end-of-combat trigger stacked above it.
func TestRegularCombatDamageTriggerResolvesInTheDamageStep(t *testing.T) {
	t.Parallel()
	reg := testutil.CorpusRegistry(t)
	e, cfg := combatTriggerBoard(t, reg, nil, []string{
		"Name:Raider\nManaCost:1 R\nTypes:Creature Human\nPT:2/2\n" +
			"T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | Execute$ OnGain | TriggerDescription$ Whenever CARDNAME deals combat damage to a player, you gain 3 life.\n" +
			"SVar:OnGain:DB$ GainLife | LifeAmount$ 3\nOracle:x\n",
		"Name:Sentinel\nManaCost:1 W\nTypes:Creature Human\nPT:0/4\n" +
			"T:Mode$ Phase | Phase$ EndCombat | ValidPlayer$ You | TriggerZones$ Battlefield | Execute$ OnGain | TriggerDescription$ At end of combat, you gain 1 life.\n" +
			"SVar:OnGain:DB$ GainLife | LifeAmount$ 1\nOracle:x\n",
	}, nil, nil)
	e.emit(events.Event{Kind: events.TurnChange, Player: 0, Amount: 2})
	e.emit(events.Event{Kind: events.StepChange, Step: state.StepDeclareAttackers})
	raider := findBattlefield(t, e, 0, "Raider", 0)
	start := len(e.L.Events)

	e.askAttackers()
	submitAttackersOnly(t, e, raider)
	activeInDamageStep := false
	for i := 0; i < 64 && e.G.Step <= state.StepEndCombat; i++ {
		d := e.Pending()
		if d == nil {
			t.Fatal("no pending decision")
		}
		if d.Kind == decision.KBlockers {
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player}); err != nil {
				t.Fatalf("no blocks: %v", err)
			}
			continue
		}
		if d.Kind == decision.KTriggerOrder {
			// Only reachable when both triggers wait together (the bug):
			// answer in offered order so the assertions below report it.
			picks := make([]int, len(d.Options))
			for j, o := range d.Options {
				picks[j] = o.Index
			}
			if err := e.Submit(decision.Intent{Seq: d.Seq, Player: d.Player, Choices: picks}); err != nil {
				t.Fatalf("trigger order: %v", err)
			}
			continue
		}
		if d.Kind != decision.KPriority {
			t.Fatalf("unexpected %s decision", d.Kind)
		}
		if e.G.Step == state.StepCombatDamage && e.G.Players[1].Life < 20 && d.Player == e.G.Active {
			activeInDamageStep = true
		}
		submitPass(t, e)
	}
	if e.G.Players[1].Life != 18 {
		t.Fatalf("defender life %d, want 18 (the Raider's damage)", e.G.Players[1].Life)
	}
	if !activeInDamageStep {
		t.Fatal("the active player never received priority in the combat damage step after the regular damage (CR 510.3)")
	}
	// Event order: the damage trigger's +3 resolves inside the combat damage
	// step, then the end of combat step begins, then its +1 resolves.
	step := state.StepDeclareAttackers
	var gains []struct {
		amount int32
		step   state.Step
	}
	for _, ev := range e.L.Events[start:] {
		switch {
		case ev.Kind == events.StepChange:
			step = ev.Step
		case ev.Kind == events.LifeChange && ev.Player == 0 && ev.Amount > 0:
			gains = append(gains, struct {
				amount int32
				step   state.Step
			}{ev.Amount, step})
		}
	}
	if len(gains) != 2 || gains[0].amount != 3 || gains[0].step != state.StepCombatDamage ||
		gains[1].amount != 1 || gains[1].step != state.StepEndCombat {
		t.Fatalf("life gains %+v, want +3 in the combat damage step, then +1 in end of combat", gains)
	}
	replayCheck(t, e, cfg)
}
