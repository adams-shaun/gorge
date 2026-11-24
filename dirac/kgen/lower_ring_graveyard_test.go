package kgen

import "testing"

// TestLowerRingTemptsYouGraveyard lowers Ringwraiths from the corpus: its
// "When the Ring tempts you, return CARDNAME from your graveyard to your
// hand" line is a Mode$ RingTemptsYou trigger whose TriggerZones$ is
// Graveyard. The lowering must accept that zone and keep the line (the card
// still carries its ETB trigger too), which is what the kernel's graveyard
// walk (dirac/act_ring.go fireRingTempt) needs.
func TestLowerRingTemptsYouGraveyard(t *testing.T) {
	c := lowerCorpusCard(t, "Ringwraiths")
	for _, rf := range c.Refusals {
		t.Errorf("Ringwraiths refused at %s: %s", rf.Node, rf.Reason)
	}
	var etb, grave bool
	for _, a := range c.Abilities {
		if a.Kind != akTriggered || a.Trigger == nil {
			continue
		}
		switch a.Trigger.Event {
		case "ETB":
			etb = true // precondition: the card's other printed trigger lowered
		case "RingTemptsYou":
			if a.Trigger.Zone != zGraveyard {
				t.Errorf("RingTemptsYou trigger zone = %d, want the graveyard (%d)", a.Trigger.Zone, zGraveyard)
				continue
			}
			if a.Trigger.Whose != whoYou {
				t.Errorf("RingTemptsYou ValidPlayer lowered to whose %d, want whoYou", a.Trigger.Whose)
			}
			grave = true
		}
	}
	if !etb {
		t.Fatal("Ringwraiths' ETB trigger did not lower: the card was not read as expected")
	}
	if !grave {
		t.Fatal("Ringwraiths has no graveyard Mode$ RingTemptsYou trigger")
	}
}
