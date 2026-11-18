package kgen

import (
	"strings"
	"testing"
)

// TestRavenloftAdventurerLowers: the RememberChanged$ + PutCounter HIT chain
// lowers to ExileOnLeave with the HIT kind. The card still refuses by name on
// its Attacks trigger: dirac has no completed-dungeon count (DungeonsCompleted$
// Amount), so that reason is the only one left.
func TestRavenloftAdventurerLowers(t *testing.T) {
	cs := lowerNamed(t, []string{"Ravenloft Adventurer"})
	if len(cs) != 1 {
		t.Fatalf("got %d cards", len(cs))
	}
	c := cs[0]
	var rs []string
	for _, r := range c.Refusals {
		rs = append(rs, r.Reason)
	}
	for _, r := range rs {
		if strings.Contains(r, exileCounterRiderReason) {
			t.Errorf("the counter rider chain is still refused: %q", rs)
		}
	}
	if len(rs) != 1 || !strings.Contains(rs[0], "DungeonsCompleted$") {
		t.Errorf("refusals = %q, want only the DungeonsCompleted$ value head", rs)
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
