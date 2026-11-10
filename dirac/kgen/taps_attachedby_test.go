package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestTapsAttachedByDefer: Cryoshatter's "when enchanted creature becomes
// tapped, destroy it" (ValidCard$ Card.AttachedBy, no ValidPlayer$) lowers as
// a deferring Taps watcher whose body destroys the trigger object, as gorge's
// planner defers the enchanted creature's mana tap; an AttachedBy watcher
// whose body damages the payer (Psychic Venom's shape) still refuses.
func TestTapsAttachedByDefer(t *testing.T) {
	cs := lowerNamed(t, []string{"Cryoshatter"})
	if len(cs) != 1 {
		t.Fatalf("lowered %d cards, want Cryoshatter", len(cs))
	}
	c := cs[0]
	if len(c.Refusals) > 0 {
		t.Fatalf("%s refused: %v", c.Name, c.Refusals)
	}
	found := false
	for _, a := range c.Abilities {
		tr := a.Trigger
		if tr == nil || tr.Event != "Taps" {
			continue
		}
		found = true
		if tr.F == nil || len(tr.F.Clauses) != 1 || tr.F.Clauses[0] != (lClause{Flags: fEquippedBy}) {
			t.Errorf("trigger filter %v, want one AttachedBy clause", tr.F)
		}
		if tr.Whose != whoAny {
			t.Errorf("Whose %d, want whoAny (any tapper)", tr.Whose)
		}
		if tr.TapPlan != uint8(kop.TapPlanDefer) {
			t.Errorf("TapPlan %d, want Defer", tr.TapPlan)
		}
		destroy := false
		for _, in := range a.Prog {
			if strings.HasPrefix(in.Op, "Destroy") && in.Obj.Kind == rTriggered {
				destroy = true
			}
		}
		if !destroy {
			t.Errorf("no destroy of the trigger object: %v", a.Prog)
		}
	}
	if !found {
		t.Fatal("no Taps trigger")
	}
	const line = "T:Mode$ Taps | ValidCard$ Card.AttachedBy | TriggerZones$ Battlefield | Execute$ TrigX | TriggerDescription$ x"
	for _, tc := range []struct {
		name, sv string
		refuse   bool
	}{
		{"destroy", "SVar:TrigX:DB$ Destroy | Defined$ TriggeredCardLKICopy", false},
		{"damage to the tapped permanent's controller", "SVar:TrigX:DB$ DealDamage | Defined$ TriggeredCardController | NumDmg$ 2", true},
		{"damage to an opponent", "SVar:TrigX:DB$ DealDamage | Defined$ Opponent | NumDmg$ 1", true},
	} {
		why, _, _ := ktqBuildSV(t, line, tc.sv)
		if got := strings.Contains(why, "Taps ValidCard"); got != tc.refuse {
			t.Errorf("%s: refusal %q, want refused=%v", tc.name, why, tc.refuse)
		}
	}
}
