package kgen

import (
	"strings"
	"testing"
)

// TestChooseCardMandatoryControlledByPlayerRefuses: ControlledByPlayer$ on
// a mandatory ChooseCard refuses unless it is Chooser, the one value the
// lowering spells in the kernel's pool base (choosecard_poolctrl_test.go);
// accepting another would silently draw the candidates from every seat. The
// base shape lowers, which is the precondition the refusal asserts.
func TestChooseCardMandatoryControlledByPlayerRefuses(t *testing.T) {
	const base = "Name:ChooseCard Probe\nManaCost:1 U\nTypes:Sorcery\nA:SP$ ChooseCard | Mandatory$ True | Choices$ Creature"
	if got := keclReason(keclSynth(t, "ChooseCard Probe", base+"\n")); got != "" {
		t.Fatalf("the base mandatory ChooseCard did not lower: %s", got)
	}
	for _, v := range []string{"Opponent", "Left", "Remembered", "Right"} {
		got := keclReason(keclSynth(t, "ChooseCard Probe", base+" | ControlledByPlayer$ "+v+"\n"))
		if !strings.Contains(got, "mandatory ChooseCard ControlledByPlayer$") {
			t.Errorf("ControlledByPlayer$ %s: refusals %q, want one containing the mandatory refusal", v, got)
		}
	}
}
