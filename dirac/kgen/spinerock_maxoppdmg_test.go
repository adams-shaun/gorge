package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSpinerockKnollMaxOppDamage: Spinerock Knoll's Hideaway Play gates on
// "if an opponent was dealt 7 or more damage this turn"
// (ConditionCheckSVar$ X GE7, SVar:X:Count$MaxOppDamageThisTurn). The value
// head must lower to kop.ExMaxOppDamageThisTurn and reach the Play's SkipIf,
// or the whole card is refused at the head.
func TestSpinerockKnollMaxOppDamage(t *testing.T) {
	c := lowerCorpusCard(t, "Spinerock Knoll")
	for _, rf := range c.Refusals {
		t.Errorf("Spinerock Knoll refused: %s", rf.Reason)
	}
	if got := exprKinds[vMaxOppDmgTaken]; got != kop.ExMaxOppDamageThisTurn {
		t.Fatalf("exprKinds[vMaxOppDmgTaken] = %v, want kop.ExMaxOppDamageThisTurn", got)
	}
	var gated bool
	for _, a := range c.Abilities {
		for _, in := range a.Prog {
			if in.Op != "SkipIf" || in.Cond.A.Kind != vMaxOppDmgTaken {
				continue
			}
			if in.Cond.Op != opGE || !in.Cond.B.isConst() || in.Cond.B.N != 7 {
				t.Errorf("Play gate = %s, want the max damage GE 7", in.Cond)
			}
			gated = true
		}
	}
	if !gated {
		t.Fatal("Spinerock Knoll: no SkipIf over MaxOppDamageThisTurn in any ability")
	}
}
