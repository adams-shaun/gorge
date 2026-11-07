package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/engine/decision"
)

// TestAdaptiveAutomatonEntryTriggersAnotherAngel: the Automaton's as-enters
// type choice is part of the permanent a "whenever another Angel you control
// enters" trigger checks (CR 614.12, 603.6a).
func TestAdaptiveAutomatonEntryTriggersAnotherAngel(t *testing.T) {
	t.Parallel()
	auto := "Name:Adaptive Automaton\nManaCost:3\nTypes:Artifact Creature Construct\nPT:2/2\n" +
		"K:ETBReplacement:Other:PickKind\nSVar:PickKind:DB$ ChooseType | Type$ Creature | AILogic$ MostProminentInComputerDeck\n" +
		"S:Mode$ Continuous | AddType$ ChosenType | Affected$ Card.Self\nOracle:x\n"
	val := "Name:Youthful Valkyrie\nManaCost:2 W\nTypes:Creature Angel\nPT:1/3\nK:Flying\n" +
		"T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Angel.Other+YouCtrl | TriggerZones$ Battlefield | Execute$ TrigPut | TriggerDescription$ x\n" +
		"SVar:TrigPut:DB$ PutCounter | Defined$ Self | CounterType$ P1P1 | CounterNum$ 1\nOracle:x\n"
	for _, tc := range []struct {
		pick string
		want int
	}{{"Angel", 1}, {"Goblin", 0}} {
		e, cfg, _ := newFixtureDeck(t, 6207, auto, val)
		vid := putCreature(t, e, 0, val)
		addMana(t, e, 0, "CCC")
		castFirst(t, e, "cast")
		d := passUntilNonPriority(t, e, 40)
		if d == nil || d.Kind != decision.KChoose || d.Options[0].Kind != "type" {
			t.Fatalf("type choice = %+v", d)
		}
		idx := -1
		for _, o := range d.Options {
			if o.Label == tc.pick {
				idx = o.Index
			}
		}
		if idx < 0 {
			t.Fatalf("%s not offered", tc.pick)
		}
		submitChoices(t, e, idx)
		passUntilStackEmpty(t, e, 20)
		if got := int(e.Power(vid)) - 1; got != tc.want {
			t.Fatalf("%s: valkyrie +1/+1 counters = %d, want %d", tc.pick, got, tc.want)
		}
		replayCheck(t, e, cfg)
	}
}
