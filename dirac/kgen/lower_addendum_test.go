package kgen

import (
	"strings"
	"testing"
)

// addendumScript is a synthetic instant carrying the Addendum gate (Tail
// Swipe's, Code of Constraint's): its sub-ability runs only when the spell
// was cast during its controller's main phase.
const addendumScript = `Name:Test Addendum
ManaCost:G
Types:Instant
A:SP$ Draw | NumCards$ 1 | SubAbility$ DBGain | SpellDescription$ Draw a card.
SVar:DBGain:DB$ GainLife | LifeAmount$ 2 | ConditionPlayerTurn$ True | ConditionPhases$ Main1,Main2 | ConditionDefined$ Self | ConditionPresent$ Card.wasCast | SpellDescription$ Addendum gain 2 life.
`

// addendumAbilityScript puts the same gate on an activated ability: the
// source is a permanent, not the resolving spell, so the cast read refuses.
const addendumAbilityScript = `Name:Test Addendum Relic
ManaCost:1
Types:Artifact
A:AB$ GainLife | Cost$ T | LifeAmount$ 2 | ConditionPlayerTurn$ True | ConditionPhases$ Main1,Main2 | ConditionDefined$ Self | ConditionPresent$ Card.wasCast | SpellDescription$ Gain 2 life.
`

// TestAddendumGateLowers: ConditionPlayerTurn$+ConditionPhases$ beside a
// ConditionDefined$ Self | ConditionPresent$ Card.wasCast group lowers to
// the turn/phase test AND the resolving spell's own cast read; the same gate
// off a spell refuses; and a gate beside further Condition*$ keys refuses on
// the first of them in sorted order, run after run.
func TestAddendumGateLowers(t *testing.T) {
	t.Run("spell lowers to the conjunction", func(t *testing.T) {
		c := lowerScriptCard(t, "Test Addendum", addendumScript)
		if len(c.Refusals) != 0 {
			t.Fatalf("refused: %v", c.Refusals)
		}
		if len(c.Abilities) != 1 {
			t.Fatalf("%d abilities, want 1", len(c.Abilities))
		}
		var gate *lInstr
		for i := range c.Abilities[0].Prog {
			if in := &c.Abilities[0].Prog[i]; in.Op == "SkipIf" {
				gate = in
			}
		}
		if gate == nil {
			t.Fatalf("no SkipIf in %v", c.Abilities[0].Prog)
		}
		cond := gate.Cond
		if cond.Kind != cAnd || len(cond.Sub) != 2 {
			t.Fatalf("gate %v, want (turn AND phases) AND cast", cond)
		}
		turn, cast := cond.Sub[0], cond.Sub[1]
		if turn.Kind != cAnd || len(turn.Sub) != 2 || turn.Sub[0].Kind != cYourTurn || turn.Sub[0].Neg || turn.Sub[1].A.Kind != vStepIn {
			t.Fatalf("turn/phase operand %v, want YourTurn AND StepIn", turn)
		}
		if cast.Kind != cRefMatches || len(cast.F.Clauses) != 1 || cast.F.Clauses[0].CmpStat != cmpWasCastSpell || cast.F.Clauses[0].CmpN != spellCast {
			t.Fatalf("cast operand %v, want the resolving spell's own cast read", cast)
		}
	})

	t.Run("off a spell refuses", func(t *testing.T) {
		c := lowerScriptCard(t, "Test Addendum Relic", addendumAbilityScript)
		if len(c.Refusals) == 0 || !strings.Contains(c.Refusals[0].Reason, "wasCast") {
			t.Fatalf("refusals %v, want the wasCast read refused off a spell", c.Refusals)
		}
	})

	t.Run("more keys refuse on the sorted first", func(t *testing.T) {
		for range 10 {
			lc := &lowerCtx{}
			ps := newParams(map[string]string{
				"ConditionPlayerTurn": "True",
				"ConditionPhases":     "Main1,Main2",
				"ConditionPresent":    "Card.ChosenCard",
				"ConditionDefined":    "Remembered",
				"ConditionCompare":    "EQ1",
			})
			if _, err := lc.playerTurnCondition(ps, "True"); err == nil || !strings.Contains(err.Error(), "ConditionPlayerTurn$ beside ConditionCompare$") {
				t.Fatalf("err %v, want the sorted-first ConditionCompare$", err)
			}
		}
	})
}
