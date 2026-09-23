package cards

import "testing"

func TestVanishingExpansion(t *testing.T) {
	f := expanded(t, "Name:V\nManaCost:3\nTypes:Creature\nPT:1/1\nK:Vanishing:3\nOracle:x\n")
	if len(f.Repls) != 1 || f.Repls[0].Params["Keyword"] != "Vanishing" || f.Repls[0].With == nil ||
		f.Repls[0].With.API != "PutCounter" || f.Repls[0].With.Params["CounterType"] != "TIME" || f.Repls[0].With.Params["CounterNum"] != "3" {
		t.Fatalf("Vanishing entry replacement = %+v, want ETB placement of 3 TIME counters", f.Repls)
	}
	if len(f.Triggers) != 2 {
		t.Fatalf("Vanishing triggers = %d, want upkeep removal and last-counter sacrifice", len(f.Triggers))
	}
	if f.Triggers[0].Mode != "Phase" || f.Triggers[0].Params["Phase"] != "Upkeep" ||
		f.Triggers[0].Params["ValidPlayer"] != "You" || f.Triggers[0].Params["Execute"] == "" {
		t.Fatalf("Vanishing upkeep trigger = %+v", f.Triggers[0])
	}
	if f.Triggers[1].Mode != "CounterRemoved" || f.Triggers[1].Params["CounterType"] != "TIME" ||
		f.Triggers[1].Params["NewCounterAmount"] != "0" || f.Triggers[1].Params["TriggerZones"] != "Battlefield" {
		t.Fatalf("Vanishing last-counter trigger = %+v", f.Triggers[1])
	}
}
