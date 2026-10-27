package effects

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/state"
)

func TestConditionManaSpentReadsColorSpent(t *testing.T) {
	h, c := fixtureHost(t)
	src := h.g.Obj(c.Source)
	check := func(line string, want bool) {
		t.Helper()
		got, resolved := conditionMet(h, c, sa(t, "DB$ GainLife | LifeAmount$ 2 | "+line))
		if !resolved || got != want {
			t.Errorf("%s = (%v, resolved %v), want (%v, true)", line, got, resolved, want)
		}
	}
	src.ManaColorSpent[state.MG] = 1
	if src.ManaColorSpent[state.MG] != 1 || src.ManaColorSpent[state.MW] != 0 {
		t.Fatalf("precondition: vector %v", src.ManaColorSpent)
	}
	check("ConditionManaSpent$ G", true)
	check("ConditionManaNotSpent$ G", false)
	check("ConditionManaSpent$ G W", false)
	check("ConditionManaNotSpent$ W", true)

	src.ManaColorSpent = state.Mana{}
	check("ConditionManaSpent$ G", false)
	check("ConditionManaNotSpent$ G", true)

	c.Source = 0
	check("ConditionManaSpent$ G", false)
	check("ConditionManaNotSpent$ G", true)
}
