package kgen

import (
	"strings"
	"testing"
)

// TestEmblemTriggeredCardModes: the RememberObjects$ TriggeredCard capture is
// read only over a Phase or ChangesZone Effect trigger, the two events whose
// emblem hooks hand the captured card to the trigger (dirac fires
// fireEmblemsAmt and fireEmblemsObj). An Effect trigger on any other event
// keeps refusing by name, so no card lowers with a capture the kernel cannot
// carry to its body.
func TestEmblemTriggeredCardModes(t *testing.T) {
	const probe = `Name:Triggered Probe
ManaCost:2 U
Types:Creature Human
PT:2/2
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigEffect | TriggerDescription$ probe
SVar:TrigEffect:DB$ Effect | Triggers$ TrigEOT | RememberObjects$ TriggeredCard | Duration$ Permanent
SVar:TrigEOT:Mode$ Phase | Phase$ End of Turn | Execute$ TrigReturn
SVar:TrigReturn:DB$ ChangeZone | Defined$ Remembered | Origin$ Graveyard | Destination$ Hand
Oracle:probe
`
	if got := lowerScript(t, "Triggered Probe", probe); len(got) != 0 {
		t.Errorf("TriggeredCard over Phase: refused %v, want the accepted capture", got)
	}

	other := strings.Replace(probe, "Mode$ Phase | Phase$ End of Turn", "Mode$ SpellCast | ValidCard$ Card", 1)
	got := strings.Join(lowerScript(t, "Triggered Probe", other), "\n")
	if !strings.Contains(got, `Effect triggers with RememberObjects$ "TriggeredCard"`) {
		t.Errorf("TriggeredCard over SpellCast: refusals %q, want the by-name refusal", got)
	}

	// ChangesZone: the emblem trigger's event object is the card that left,
	// not the parent trigger's card the Effect remembered (Lucius: the
	// imprinted creature leaves, the capture is Lucius). The self-exiling
	// body is the Effect's end, so the emblem carries Once and the body is
	// dropped (the emblem performs the end).
	const zoneProbe = `Name:Triggered Zone Probe
ManaCost:2 U
Types:Creature Human
PT:2/2
T:Mode$ ChangesZone | Origin$ Any | Destination$ Battlefield | ValidCard$ Card.Self | Execute$ TrigEffect | TriggerDescription$ probe
SVar:TrigEffect:DB$ Effect | Triggers$ TrigLeave | RememberObjects$ TriggeredCard | Duration$ Permanent
SVar:TrigLeave:Mode$ ChangesZone | Origin$ Battlefield | Destination$ Any | ValidCard$ Creature | Execute$ TrigReturn
SVar:TrigReturn:DB$ ChangeZone | Defined$ Remembered | Origin$ Graveyard | Destination$ Hand | SubAbility$ ExileSelf
SVar:ExileSelf:DB$ ChangeZone | Destination$ Exile | Origin$ Command | Defined$ Self
Oracle:probe
`
	card := lowerScriptCard(t, "Triggered Zone Probe", zoneProbe)
	for _, rf := range card.Refusals {
		t.Errorf("TriggeredCard over ChangesZone: refused %s, want the accepted capture", rf.Reason)
	}
	if len(card.Emblems) != 1 {
		t.Fatalf("TriggeredCard over ChangesZone: %d emblems, want 1", len(card.Emblems))
	}
	if em := card.Emblems[0]; !em.CaptureSet || em.Capture.Kind != rTriggered || !em.Once {
		t.Errorf("TriggeredCard over ChangesZone emblem = %+v, want Capture=Triggered and Once", em)
	}
}
