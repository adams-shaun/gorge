package kgen

import (
	"strings"
	"testing"
)

// TestPlayValidSAProbeIsNotAFilterHead: the every-blocker census probes a
// refused node's unread Valid* params as filters, but a Play's ValidSA$ is
// read by playSAExpr, not as a card filter. Kuroki, Massimo, Plargg and
// Nassari, Tinybones and God-Eternal Kefnet all carried a false "Spell head
// outside a ReduceCost ValidTarget$" blocker from ValidSA$ Spell.
func TestPlayValidSAProbeIsNotAFilterHead(t *testing.T) {
	const spellHead = "a Spell head outside a ReduceCost ValidTarget$"
	// ManaConversion$ refuses the Play before ValidSA$ is read.
	src := func(validSA string) string {
		return strings.Join([]string{
			"Name:Probe",
			"ManaCost:B",
			"Types:Creature Skeleton",
			"PT:1/1",
			"T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | Execute$ TrigCast | TriggerDescription$ Probe.",
			"SVar:TrigCast:DB$ Play | ValidTgts$ Permanent.nonLand+ControlledBy TriggeredTarget | TgtZone$ Graveyard | ValidSA$ " + validSA + " | Optional$ True | ManaConversion$ AnyType->AnyType",
		}, "\n")
	}
	all := abReasons(abLower(t, src("Spell"), true))
	if !strings.Contains(all, "Play ManaConversion") {
		t.Fatalf("precondition: the Play must refuse on ManaConversion first:\n%s", all)
	}
	if strings.Contains(all, spellHead) || strings.Contains(all, `filter "Spell"`) {
		t.Errorf("ValidSA$ Spell reported as a filter-head blocker:\n%s", all)
	}
	// a ValidSA$ play itself would refuse still shows as a blocker
	bad := abReasons(abLower(t, src("Bogus"), true))
	if !strings.Contains(bad, `ValidSA "Bogus"`) {
		t.Errorf("an unreadable ValidSA$ lost its blocker:\n%s", bad)
	}
}
