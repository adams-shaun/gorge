package cards

import "testing"

// TestETBReplacementFilterFieldReachesValidCard pins the keyword expansion's
// trailing colon field wiring at the compile level (task etbrepl-filter):
// the 6th content field of
//
//	K:ETBReplacement:<layer>:<SVar>:<Mandatory|Optional>:<validZone>:<filter>
//
// is the entering-card filter, and it must become the replacement's
// ValidCard$. A line with no filter field keeps the historical Card.Self
// fallback byte-identical (every no-filter carrier: ChooseColor, ChooseCT,
// DBNameCard, ...).
func TestETBReplacementFilterFieldReachesValidCard(t *testing.T) {
	// Veiled Ascension's exact keyword line.
	f := expanded(t, "Name:Veil\nManaCost:3 W\nTypes:Enchantment\n"+
		"K:ETBReplacement:Other:AddExtraCounter:Mandatory:Battlefield:Creature.faceDown+YouCtrl\n"+
		"SVar:AddExtraCounter:DB$ PutCounter | ETB$ True | Defined$ ReplacedCard | CounterType$ Flying\nOracle:x\n")
	if len(f.Repls) != 1 {
		t.Fatalf("want 1 replacement, got %d", len(f.Repls))
	}
	got := f.Repls[0].Params["ValidCard"]
	if got != "Creature.faceDown+YouCtrl" {
		t.Fatalf("ValidCard$ = %q, want the filter field", got)
	}
	if az := f.Repls[0].Params["ActiveZones"]; az != "Battlefield" {
		t.Fatalf("ActiveZones$ = %q, want Battlefield", az)
	}

	// Giada, Font of Hope's line (a non-faceDown filter).
	f = expanded(t, "Name:Giada\nManaCost:1 W\nTypes:Creature Angel\nPT:2/2\n"+
		"K:ETBReplacement:Other:AddExtraCounter:Mandatory:Battlefield:Creature.Angel+YouCtrl+Other\n"+
		"SVar:AddExtraCounter:DB$ PutCounter | ETB$ True | Defined$ ReplacedCard\nOracle:x\n")
	if got := f.Repls[0].Params["ValidCard"]; got != "Creature.Angel+YouCtrl+Other" {
		t.Fatalf("Giada ValidCard$ = %q, want the filter field", got)
	}

	// No filter field: the fallback must stay Card.Self (ChooseColor).
	f = expanded(t, "Name:Sprawl\nManaCost:G\nTypes:Enchantment Aura\n"+
		"K:ETBReplacement:Other:ChooseColor\nSVar:ChooseColor:DB$ ChooseColor\nOracle:x\n")
	if got := f.Repls[0].Params["ValidCard"]; got != "Card.Self" {
		t.Fatalf("no-filter ValidCard$ = %q, want Card.Self fallback", got)
	}
	if _, ok := f.Repls[0].Params["ActiveZones"]; ok {
		t.Fatalf("no-filter line gained ActiveZones$: %q", f.Repls[0].Params["ActiveZones"])
	}
}

// TestETBReplacementFilterFieldEmptyZoneStillReadsFilter pins the brief's
// field-position edge: an EMPTY valid-zone field still puts the filter at
// Split index 2. metathran_transport's shape
// ("...:Mandatory::Card.Self+escaped").
func TestETBReplacementFilterFieldEmptyZoneStillReadsFilter(t *testing.T) {
	f := expanded(t, "Name:Mimic\nManaCost:2\nTypes:Artifact Creature\nPT:0/0\n"+
		"K:ETBReplacement:Other:ChooseCT:Mandatory::Card.Self+escaped\n"+
		"SVar:ChooseCT:DB$ ChooseType | Type$ Creature\nOracle:x\n")
	if got := f.Repls[0].Params["ValidCard"]; got != "Card.Self+escaped" {
		t.Fatalf("empty-zone ValidCard$ = %q, want Card.Self+escaped", got)
	}
	if _, ok := f.Repls[0].Params["ActiveZones"]; ok {
		t.Fatalf("empty zone field must not set ActiveZones$: %q", f.Repls[0].Params["ActiveZones"])
	}
}
