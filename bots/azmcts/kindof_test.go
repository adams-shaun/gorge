package azmcts

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

func opts(kinds ...string) []decision.Option {
	var out []decision.Option
	for i, k := range kinds {
		out = append(out, decision.Option{Index: i, Kind: k})
	}
	return out
}

// Kinds.Searches is the predicate the seat applies before it enumerates
// candidates: the default kinds search the four classic decisions and none of
// the fuller ones, FullKinds adds modes, optional triggers, small choices and
// multi-target picks, and a nil decision is never searched.
func TestKindsSearches(t *testing.T) {
	single := &decision.Decision{Kind: decision.KTarget, Min: 1, Max: 1, Options: opts("permanent", "player")}
	multi := &decision.Decision{Kind: decision.KTarget, Min: 1, Max: 2, Options: opts("permanent", "permanent", "player")}
	modes := &decision.Decision{Kind: decision.KModes, Min: 1, Max: 1, Options: opts("mode", "mode")}
	optional := &decision.Decision{Kind: decision.KTriggerOptional, Options: opts("yes", "no")}
	choose := &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: opts("number", "number")}
	manaWindow := &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: opts("activate", "done")}
	bigChoose := &decision.Decision{Kind: decision.KChoose, Min: 1, Max: 1, Options: opts("a", "b", "c", "d", "e", "f", "g", "h")}
	priority := &decision.Decision{Kind: decision.KPriority, Options: opts("pass", "cast")}
	mulligan := &decision.Decision{Kind: decision.KMulligan, Options: opts("keep", "mulligan")}
	commanderZone := &decision.Decision{Kind: decision.KCommanderZone, Min: 1, Max: 1, Options: opts("command_zone", "leave")}

	cases := []struct {
		name        string
		d           *decision.Decision
		all, fuller bool
	}{
		{"nil", nil, false, false},
		{"priority", priority, true, true},
		{"single target", single, true, true},
		{"multi target", multi, false, true},
		{"modes", modes, false, true},
		{"optional", optional, false, true},
		{"small choose", choose, false, true},
		{"mana window", manaWindow, false, false},
		{"large choose", bigChoose, false, false},
		{"mulligan", mulligan, false, false},
		{"commander zone", commanderZone, false, false},
	}
	for _, c := range cases {
		if got := AllKinds().Searches(c.d); got != c.all {
			t.Errorf("AllKinds().Searches(%s) = %v, want %v", c.name, got, c.all)
		}
		if got := FullKinds().Searches(c.d); got != c.fuller {
			t.Errorf("FullKinds().Searches(%s) = %v, want %v", c.name, got, c.fuller)
		}
	}
	if !(Kinds{Mulligan: true}).Searches(mulligan) {
		t.Error("a Kinds with Mulligan does not search the mulligan")
	}
	if !(Kinds{CommanderZone: true}).Searches(commanderZone) {
		t.Error("a Kinds with CommanderZone does not search the commander-zone choice")
	}
	if (Kinds{}).Searches(priority) {
		t.Error("the zero Kinds searches a priority decision")
	}
}
