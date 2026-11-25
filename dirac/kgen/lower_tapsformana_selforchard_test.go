package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestLowerTapsformanaSelfOrchard: Forbidden Orchard's "Whenever you tap
// CARDNAME for mana, target opponent creates a 1/1 colorless Spirit creature
// token" is a self-line TapsForMana watcher whose body the planner defers
// (kop.TapPlanDefer). The lowering keeps the bare Card.Self clause (the
// builder turns it into kop.TfSelf), accepts Activator$ You, and carries the
// token body's own ValidTgts$ Opponent as one player target slot. Before the
// self shape lowered, the card was refused by name ("trigger mode
// TapsForMana").
func TestLowerTapsformanaSelfOrchard(t *testing.T) {
	var c *lCard
	for _, card := range lowerNamed(t, []string{"Forbidden Orchard"}) {
		if card.Name == "Forbidden Orchard" {
			c = card
		}
	}
	if c == nil {
		t.Fatal("Forbidden Orchard: not lowered")
	}
	if len(c.Refusals) != 0 {
		t.Fatalf("Forbidden Orchard refused: %v", c.Refusals)
	}
	lines := 0
	for i := range c.Abilities {
		ab := c.Abilities[i]
		tr := ab.Trigger
		if tr == nil || !tr.TapsForMana {
			continue
		}
		lines++
		if tr.Event != "Taps" || tr.TapPlan != uint8(kop.TapPlanDefer) {
			t.Errorf("Taps line event %q plan %d, want Taps/Defer", tr.Event, tr.TapPlan)
		}
		if tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Clauses[0].Flags != fSelf {
			t.Errorf("Taps line filter %+v, want the bare Card.Self clause", tr.F)
		}
		if tr.TapperNonActive {
			t.Error("Taps line reads Activator$ Player.NonActive, want Activator$ You")
		}
		if len(ab.Targets) != 1 {
			t.Fatalf("token body targets = %d, want 1", len(ab.Targets))
		}
		tg := ab.Targets[0]
		if tg.F == nil || tg.F.Players != whoOpp || len(tg.F.Clauses) != 0 {
			t.Errorf("token target %+v, want the token's own ValidTgts$ Opponent player slot", tg)
		}
	}
	if lines != 1 {
		t.Errorf("Forbidden Orchard: %d TapsForMana lines, want 1", lines)
	}
}
