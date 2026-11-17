package kgen

import "testing"

// manifestVariantForms: the Manifest Defined$ ValidLibrary, Choices$ hand and
// Amount$ SVar shapes gorge's effManifest plays (zone_manifest.go).
var manifestVariantForms = []formCase{
	{card: "Orochi Soul-Reaver", form: "Defined$ ValidLibrary Card.TopLibrary+ControlledBy TriggeredTarget", lir: "Manifest"},
	{card: "Thieving Amalgam", form: "Defined$ ValidLibrary Card.TopLibrary+ActivePlayerCtrl", lir: "Manifest"},
	{card: "Scroll of Fate", form: "Choices$ Card.YouCtrl | ChoiceZone$ Hand", lir: "CS=FaceDownPick"},
	{card: "Omarthis, Ghostfire Initiate", form: "Amount$ Y (TriggeredCard counters)", lir: "Manifest"},
}

func TestManifestVariantForms(t *testing.T) { checkForms(t, manifestVariantForms) }
