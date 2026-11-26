package kgen

import (
	"strings"
	"testing"
)

// TestSweepFromRefusalOrder: a DamageAll DamageSource$ whose source shape the
// sweep cannot lower refuses by its DamageSource$ value, not by the
// ValidPlayers$ player scope beside it (lower_sweepfrom.go resolves the
// source before the player arm, where the plain ValidPlayers refusal sat).
// The five corpus carriers, their printed scripts (cli-20261125T191555Z-
// fdbcc0d7's round-2 finding): each pairs the named source with a player
// scope (ValidPlayers$ Opponent or OppNonTriggeredTarget) the lowering does
// not take.
func TestSweepFromRefusalOrder(t *testing.T) {
	want := []struct{ card, reason string }{
		{"Aspiring Champion", `DamageAll DamageSource$ "Remembered"`},
		{"Human Torch", `DamageAll DamageSource$ "EffectSource"`},
		{"Kediss, Emberclaw Familiar", `DamageAll DamageSource$ "TriggeredSource"`},
		{"Pain for All", `DamageAll DamageSource$ "TriggeredTargetLKICopy"`},
		{"Super State", `DamageAll DamageSource$ "TriggeredSource"`},
	}
	var names []string
	for _, w := range want {
		names = append(names, w.card)
	}
	lw := lowerNamedPool(t, names)
	byName := map[string]*lCard{}
	for _, c := range lw.cards {
		byName[c.Name] = c
	}
	for _, w := range want {
		c := byName[w.card]
		if c == nil {
			t.Fatalf("%s: not lowered", w.card)
		}
		var found bool
		for _, r := range c.Refusals {
			// a refusal's reason carries its node's context ("DB$ DamageAll: …");
			// the class the census tables is the reason's tail
			if strings.Contains(r.Reason, "DamageAll DamageSource$ beside ValidPlayers") {
				t.Errorf("%s: the generic ValidPlayers refusal, want %q", w.card, w.reason)
			}
			found = found || strings.Contains(r.Reason, w.reason)
		}
		if !found {
			t.Errorf("%s: no %q among %q", w.card, w.reason, abReasons(c))
		}
	}
}
