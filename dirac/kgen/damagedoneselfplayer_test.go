package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDamageDoneSelfPlayerReferent: a plain DamageDone line from the card
// itself to any player (Abyssal Specter) whose body reads the damaged player
// lowers as the per-referent trigger, whose trigger player is the damaged one;
// a combat-only line keeps the plain trigger, and a DamageDoneOnce line's read
// of the damaged player stays refused.
func TestDamageDoneSelfPlayerReferent(t *testing.T) {
	const body = "SVar:Trig:DB$ Discard | Defined$ TriggeredTarget | Mode$ TgtChoose"
	why, ab, _ := ktqBuildSV(t, "T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | TriggerZones$ Battlefield | Execute$ Trig", body)
	if why != "" {
		t.Fatalf("any player: refusal %q", why)
	}
	if want := kop.Tf2PerReferent | kop.Tf2Batch; ab.Trig.Flags2&want != want {
		t.Errorf("any player: flags2 %#x, want Tf2PerReferent|Tf2Batch", ab.Trig.Flags2)
	}
	why, ab, _ = ktqBuildSV(t, "T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Player | CombatDamage$ True | TriggerZones$ Battlefield | Execute$ Trig", body)
	if why != "" {
		t.Fatalf("combat: refusal %q", why)
	}
	if ab.Trig.Flags2&kop.Tf2PerReferent != 0 {
		t.Errorf("combat: flags2 %#x, want the plain trigger", ab.Trig.Flags2)
	}
	why, _, _ = ktqBuildSV(t, "T:Mode$ DamageDone | ValidSource$ Creature | ValidTarget$ Player | TriggerZones$ Battlefield | Execute$ Trig", body)
	if why == "" {
		t.Error("non-Self source: lowered, want the triggered-target read refused")
	}
}
