package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

const wallOfBoneScript = "Name:Wall of Bone\nManaCost:2 B\nTypes:Creature Skeleton Wall\nPT:1/4\nK:Defender\nOracle:x\n"

func swampScript() string {
	return "Name:Swamp\nTypes:Basic Land Swamp\nA:AB$ Mana | Cost$ T | Produced$ B\nOracle:x\n"
}

// An adapted Incubation Druid beside only tapped lands is a priced potential cast
// of {2}{B}: its three mana are all of the one type it reflects (CR 106.1b), a
// Swamp among the lands (tapped or not) makes that {B}. The potential bound read
// the Druid as {C} alone and priced the cast out (tail-55 pair 60 decision 523).
func TestWallOfBoneIsAPricedPotentialCastFromAnAdaptedDruidAlone(t *testing.T) {
	t.Parallel()
	for _, extra := range []string{"", "forest"} {
		e := handEngine(t, card(t, wallOfBoneScript))
		druid := onBoard(t, e, 0, incubationDruidScript)
		e.G.Obj(druid).SummonSick = false
		sw := onBoard(t, e, 0, swampScript())
		e.G.Obj(sw).Tapped = true
		if extra != "" {
			fo := onBoard(t, e, 0, forestScript())
			e.G.Obj(fo).Tapped = true
		}
		e.emit(events.Event{Kind: events.CounterChange, Obj: druid, Counter: "P1P1", Amount: 3})
		e.priorityRound()
		wall := e.G.Zone(state.ZHand, 0)[0]
		found := false
		for _, pp := range e.PotentialPaymentPlans(0) {
			if pp.Action.Kind == "cast" && state.ObjID(pp.Action.Obj) == wall {
				found = true
				if pp.Reason != "" || len(pp.Script) == 0 {
					t.Fatalf("extra=%q: reason %q detail %q script %d, want a priced script", extra, pp.Reason, pp.Detail, len(pp.Script))
				}
			}
		}
		if !found {
			t.Fatalf("extra=%q: Wall of Bone is not a potential play", extra)
		}
	}
}
