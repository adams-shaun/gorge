package cards

import "testing"

// Bare K:Vanishing (Out of Time, Tidewalker) supplies its own counter
// placement, so the expansion must emit the upkeep removal and last-counter
// sacrifice triggers but NO fixed-count entry replacement.
func TestVanishingBareExpansion(t *testing.T) {
	f := expanded(t, "Name:V\nManaCost:3\nTypes:Enchantment\nK:Vanishing\nOracle:x\n")

	if len(f.Repls) != 0 {
		t.Fatalf("bare Vanishing Repls = %d, want zero (no fixed-count ETB placement): %+v", len(f.Repls), f.Repls)
	}
	if len(f.Triggers) != 2 {
		t.Fatalf("bare Vanishing triggers = %d, want upkeep removal and last-counter sacrifice", len(f.Triggers))
	}
	up := f.Triggers[0]
	if up.Mode != "Phase" || up.Params["Phase"] != "Upkeep" ||
		up.Params["ValidPlayer"] != "You" || up.Params["Execute"] == "" ||
		up.Params["Keyword"] != "Vanishing" {
		t.Fatalf("bare Vanishing upkeep trigger = %+v", up)
	}
	last := f.Triggers[1]
	if last.Mode != "CounterRemoved" || last.Params["CounterType"] != "TIME" ||
		last.Params["NewCounterAmount"] != "0" || last.Params["TriggerZones"] != "Battlefield" ||
		last.Params["Execute"] == "" || last.Params["Keyword"] != "Vanishing" {
		t.Fatalf("bare Vanishing last-counter trigger = %+v", last)
	}
}

// The numeric form keeps its fixed-count entry replacement alongside the two
// triggers. This duplicates the intent of TestVanishingExpansion as a guard
// that moving the early return did not change numeric behaviour.
func TestVanishingNumericKeepsEntryReplacement(t *testing.T) {
	f := expanded(t, "Name:V\nManaCost:3\nTypes:Creature\nPT:1/1\nK:Vanishing:3\nOracle:x\n")
	if len(f.Repls) != 1 || f.Repls[0].Params["Keyword"] != "Vanishing" ||
		f.Repls[0].With == nil || f.Repls[0].With.API != "PutCounter" ||
		f.Repls[0].With.Params["CounterType"] != "TIME" || f.Repls[0].With.Params["CounterNum"] != "3" {
		t.Fatalf("numeric Vanishing entry replacement = %+v, want ETB placement of 3 TIME counters", f.Repls)
	}
	if len(f.Triggers) != 2 {
		t.Fatalf("numeric Vanishing triggers = %d, want 2", len(f.Triggers))
	}
}
