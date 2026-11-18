package kgen

import (
	"fmt"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRavenloftAdventurerFullLowers: the whole card lowers. The dies-to-exile
// HIT chain gives ExileOnLeave with the HIT kind; the Attacks trigger's
// CheckSVar$ X reads DungeonsCompleted$Amount (the controller's completed
// dungeons, ExEnergyOf with kop.PlCtrDungeons) and its Y reader counts the
// defending player's exiled cards with a HIT counter.
func TestRavenloftAdventurerFullLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Ravenloft Adventurer"})
	if len(cs) != 1 {
		t.Fatalf("got %d cards", len(cs))
	}
	c := cs[0]
	for _, r := range c.Refusals {
		t.Errorf("refusal: %q", r.Reason)
	}
	var repl *lRepl
	for i := range c.Repls {
		if c.Repls[i].Kind == "ExileOnLeave" {
			repl = &c.Repls[i]
		}
	}
	if repl == nil {
		t.Fatalf("no ExileOnLeave repl: %+v", c.Repls)
	}
	if repl.Ctr != "HIT" || repl.CtrN != 1 {
		t.Errorf("ExileOnLeave counter = %q x%d, want HIT x1", repl.Ctr, repl.CtrN)
	}
}

// TestRavenloftAdventurerReadersAreWired: the Attacks trigger is gated on the
// completed-dungeon count (ExEnergyOf over kop.PlCtrDungeons, at least 1) and
// its life loss reads the defender's exiled HIT cards (ExCountExile owned by
// the opponent), not a constant.
func TestRavenloftAdventurerReadersAreWired(t *testing.T) {
	cs := lowerNamed(t, []string{"Ravenloft Adventurer"})
	var found bool
	for _, a := range cs[0].Abilities {
		if a.Trigger == nil || a.Trigger.Cond.Kind != cValue {
			continue
		}
		found = true
		if a.Trigger.Cond.A.Kind != vEnergyOf || a.Trigger.Cond.A.N != kop.PlCtrDungeons {
			t.Errorf("trigger gate reads %v, want the completed-dungeon count", a.Trigger.Cond.A)
		}
		if p := fmt.Sprint(a.Prog); !strings.Contains(p, "LoseLife") || !strings.Contains(p, "CountExile") {
			t.Errorf("program %s does not lose life by the exiled HIT count", p)
		}
	}
	if !found {
		t.Error("no trigger carries the CheckSVar gate")
	}
}
