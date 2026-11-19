package cards

import "testing"

// TestRemoveCounterShapeSplitsTheClass pins replPrimitive's RemoveCounter
// split: the With-less Layer$ CantHappen shape the engine implements
// (Fear of Sleep Paralysis's stun-counter lock) is the registered
// "repl:RemoveCounter"; Deification's damage arm (ReplaceWith$ ReduceLoss,
// IsDamage$ True, Result$ LT1) is repl:RemoveCounter.OtherShape and stays
// unsupported.
func TestRemoveCounterShapeSplitsTheClass(t *testing.T) {
	fear := Repl{Event: "RemoveCounter", Params: map[string]string{
		"ActiveZones":      "Battlefield",
		"ValidCard":        "Permanent.OppCtrl",
		"ValidCounterType": "STUN",
		"Layer":            "CantHappen",
		"Description":      "Stun counters can't be removed from permanents your opponents control.",
	}}
	if got := replPrimitive(nil, fear); got != "repl:RemoveCounter" {
		t.Fatalf("the implemented CantHappen shape labeled %q, want repl:RemoveCounter", got)
	}
	if !RemoveCounterCantHappenShape(fear) {
		t.Fatal("RemoveCounterCantHappenShape rejected the implemented shape")
	}
	deification := Repl{Event: "RemoveCounter", Params: map[string]string{
		"ActiveZones":      "Battlefield",
		"IsPresent":        "Creature.YouCtrl",
		"ValidCard":        "Planeswalker.YouCtrl+ChosenType",
		"Result":           "LT1",
		"ValidCounterType": "LOYALTY",
		"IsDamage":         "True",
		"ReplaceWith":      "ReduceLoss",
		"Description":      "As long as you control a creature, if damage dealt to a planeswalker you control of the chosen type would result in all loyalty counters being removed, instead remove all but one of those counters instead.",
	}}
	if got := replPrimitive(nil, deification); got != "repl:RemoveCounter.OtherShape" {
		t.Fatalf("the unimplemented damage-arm shape labeled %q, want repl:RemoveCounter.OtherShape", got)
	}
	// A With-bearing line is never the implemented shape: the class's
	// matcher rejects a body outright.
	if RemoveCounterCantHappenShape(Repl{Event: "RemoveCounter", With: &SA{Kind: "DB", API: "ReplaceEffect"},
		Params: map[string]string{"Layer": "CantHappen"}}) {
		t.Fatal("a With-bearing line passed as the CantHappen shape")
	}
	// Anything the matcher does not read fails closed.
	if RemoveCounterCantHappenShape(Repl{Event: "RemoveCounter", Params: map[string]string{
		"Layer": "CantHappen", "Result": "LT1"}}) {
		t.Fatal("an unread Result$ param passed as the implemented shape")
	}
}
