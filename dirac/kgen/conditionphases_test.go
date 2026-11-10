package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/state"
)

// TestConditionPhasesPlayerTurn: ConditionPhases$ lowers beside
// ConditionPlayerTurn$ (Great Train Heist) through the one shared phase-name
// parser, its step table maps every gorge step onto the kernel's numbering,
// and an unparseable list or a third Condition*$ key refuses.
func TestConditionPhasesPlayerTurn(t *testing.T) {
	t.Run("step table", func(t *testing.T) {
		seen := map[int]bool{}
		for st := state.Step(0); st.Valid(); st++ {
			if int(st) >= len(phaseStepBits) || phaseStepBits[st] == 0 {
				t.Fatalf("gorge step %d (%s) has no kernel mapping", st, st)
			}
			mask := phaseStepBits[st]
			if mask < 0 || mask > 1<<15-1 {
				t.Fatalf("gorge step %s mask %#x does not fit the expr's int16", st, mask)
			}
			for b := kop.Step(0); b < kop.NumSteps; b++ {
				if mask&(1<<b) == 0 {
					continue
				}
				if seen[int(b)] {
					t.Fatalf("kernel step %d is named by two gorge steps", b)
				}
				seen[int(b)] = true
			}
		}
		// gorge has one combat-damage step; both kernel damage steps carry it,
		// so every kernel step is covered exactly once.
		for b := kop.Step(0); b < kop.NumSteps; b++ {
			if !seen[int(b)] {
				t.Fatalf("kernel step %d has no gorge step", b)
			}
		}
		// BeginCombat->EndCombat is the Heist gate: five gorge steps, six
		// kernel bits (the split damage step).
		var mask int
		for st := state.StepBeginCombat; st <= state.StepEndCombat; st++ {
			mask |= phaseStepBits[st]
		}
		want := 1<<kop.StepBeginCombat | 1<<kop.StepAttackers | 1<<kop.StepBlockers |
			1<<kop.StepFirstStrikeDamage | 1<<kop.StepDamage | 1<<kop.StepEndCombat
		if mask != want {
			t.Fatalf("BeginCombat->EndCombat mask %#x, want %#x", mask, want)
		}
	})

	t.Run("great train heist lowers", func(t *testing.T) {
		checkForms(t, []formCase{{
			card: "Great Train Heist",
			form: "ConditionPhases$ BeginCombat->EndCombat beside ConditionPlayerTurn$ True (the AddPhase gate)",
			lir:  "StepIn",
		}})
	})

	t.Run("player turn and phases lower to a conjunction", func(t *testing.T) {
		lc := &lowerCtx{}
		ps := newParams(map[string]string{
			"ConditionPlayerTurn": "True",
			"ConditionPhases":     "BeginCombat->EndCombat",
		})
		c, err := lc.playerTurnCondition(ps, "True")
		if err != nil {
			t.Fatal(err)
		}
		if c.Kind != cAnd || len(c.Sub) != 2 {
			t.Fatalf("condition %v, want a two-operand conjunction", c)
		}
		if c.Sub[0].Kind != cYourTurn || c.Sub[0].Neg {
			t.Fatalf("first operand %v, want YourTurn", c.Sub[0])
		}
		want := phaseStepBits[state.StepBeginCombat] | phaseStepBits[state.StepDeclareAttackers] |
			phaseStepBits[state.StepDeclareBlockers] | phaseStepBits[state.StepCombatDamage] | phaseStepBits[state.StepEndCombat]
		if c.Sub[1].Kind != cValue || c.Sub[1].A.Kind != vStepIn || c.Sub[1].A.N != want ||
			c.Sub[1].Op != opEQ || c.Sub[1].B.Kind != vConst || c.Sub[1].B.N != 1 {
			t.Fatalf("second operand %v, want StepIn == 1 with mask %#x", c.Sub[1], want)
		}
	})

	t.Run("lone phases lowers", func(t *testing.T) {
		lc := &lowerCtx{}
		c, err := lc.phasesCondition("BeginCombat->EndCombat")
		if err != nil {
			t.Fatal(err)
		}
		if c.Kind != cValue || c.A.Kind != vStepIn || c.A.N != phaseStepBits[state.StepBeginCombat]|phaseStepBits[state.StepDeclareAttackers]|phaseStepBits[state.StepDeclareBlockers]|phaseStepBits[state.StepCombatDamage]|phaseStepBits[state.StepEndCombat] {
			t.Fatalf("lone ConditionPhases$ lowered to %v", c)
		}
	})

	t.Run("unparseable list refuses", func(t *testing.T) {
		lc := &lowerCtx{}
		ps := newParams(map[string]string{
			"ConditionPlayerTurn": "True",
			"ConditionPhases":     "BeginCombat->Bogus",
		})
		if _, err := lc.playerTurnCondition(ps, "True"); err == nil || !strings.Contains(err.Error(), "unknown phase") {
			t.Fatalf("unparseable ConditionPhases$: err %v", err)
		}
		if _, err := lc.phasesCondition("Bogus"); err == nil || !strings.Contains(err.Error(), "unknown phase") {
			t.Fatalf("lone unparseable ConditionPhases$: err %v", err)
		}
	})

	t.Run("third condition key refuses", func(t *testing.T) {
		lc := &lowerCtx{}
		ps := newParams(map[string]string{
			"ConditionPlayerTurn": "True",
			"ConditionPhases":     "BeginCombat->EndCombat",
			"ConditionPresent":    "Creature",
		})
		if _, err := lc.playerTurnCondition(ps, "True"); err == nil || !strings.Contains(err.Error(), "ConditionPlayerTurn$ beside ConditionPresent$") {
			t.Fatalf("third Condition key: err %v", err)
		}
	})
}
