package kgen

import "testing"

// exileFaceDownCarriers exile cards face down with no look (CR 406.3): a Dig
// of the controller's own library (Bomat Courier, Connecting the Dots, the
// Triumph's six) and a remembering ChangeZone / ChangeZoneAll (Induced
// Amnesia, Triumph of Saint Katherine itself), read as gorge
// applyMoveFaceDown's blind exiled_with_face_down marker.
var exileFaceDownCarriers = []string{
	"Bomat Courier", "Connecting the Dots", "Induced Amnesia", "Triumph of Saint Katherine",
}

func TestExileFaceDownLowers(t *testing.T) {
	for _, name := range exileFaceDownCarriers {
		if c := lowerNamedCard(t, name); len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
	}
}

// Inverter of Truth exiles its whole library face down without remembering
// it: the kernel marks a face-down move through the remembered set, so the
// shape stays refused (it would also outgrow the face-down link pool).
func TestExileFaceDownUnrememberedRefuses(t *testing.T) {
	if c := lowerNamedCard(t, "Inverter of Truth"); len(c.Refusals) == 0 {
		t.Error("Inverter of Truth lowers; want the ExileFaceDown$ without RememberChanged$ refusal")
	}
}
