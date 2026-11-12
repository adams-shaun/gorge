package kgen

import (
	"strings"
	"testing"
)

// TestChooseCardMandatoryControlledByPlayerRefuses: ControlledByPlayer$ on
// a mandatory ChooseCard must refuse. The kernel evaluates a mandatory pick
// from the resolving controller too, but the lowering only applied the
// chooser's own pool (YouCtrl) in the optional branch, so accepting the
// parameter would silently draw the candidates from every seat. The base
// shape lowers, which is the precondition the refusal asserts.
func TestChooseCardMandatoryControlledByPlayerRefuses(t *testing.T) {
	const base = "Name:ChooseCard Probe\nManaCost:1 U\nTypes:Sorcery\nA:SP$ ChooseCard | Mandatory$ True | Choices$ Creature"
	if got := keclReason(keclSynth(t, "ChooseCard Probe", base+"\n")); got != "" {
		t.Fatalf("the base mandatory ChooseCard did not lower: %s", got)
	}
	for _, v := range []string{"Chooser", "Opponent"} {
		got := keclReason(keclSynth(t, "ChooseCard Probe", base+" | ControlledByPlayer$ "+v+"\n"))
		if !strings.Contains(got, "mandatory ChooseCard ControlledByPlayer$") {
			t.Errorf("ControlledByPlayer$ %s: refusals %q, want one containing the mandatory refusal", v, got)
		}
	}
}
