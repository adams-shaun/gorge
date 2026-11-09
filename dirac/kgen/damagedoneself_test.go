package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestDamageDoneSelfObjectReferent: a plain DamageDone line whose ValidSource$
// is exactly the card itself and whose ValidTarget$ is an object filter, and
// whose body reads the damaged creature, lowers as the kernel's per-referent
// damage trigger (the damaged permanent is the trigger object). The same read
// from any other source keeps the plain trigger, where it refuses by name, and
// a body that reads no triggered target keeps the plain trigger too.
func TestDamageDoneSelfObjectReferent(t *testing.T) {
	// Spider-Slayer's damage trigger lowers with the per-referent flag.
	const name = "Spider-Slayer, Hatred Honed"
	all, bs := keclBuild(t, []string{name})
	i := keclIndex(t, all, name)
	if rs := lowerRefusals(all[i]); len(rs) > 0 {
		t.Fatalf("%s: refusals %v", name, rs)
	}
	var tr *kop.Trigger
	for j := range bs[i].def.Abilities {
		if a := &bs[i].def.Abilities[j]; a.Kind == kop.AbTriggered && a.Trig.Event == kop.TrigDamageDone {
			tr = &a.Trig
			break
		}
	}
	if tr == nil {
		t.Fatalf("%s: no DamageDone trigger lowered", name)
	}
	if tr.Flags2&(kop.Tf2PerReferent|kop.Tf2Batch) != kop.Tf2PerReferent|kop.Tf2Batch {
		t.Errorf("%s: trigger flags2 %#x, want Tf2PerReferent|Tf2Batch", name, tr.Flags2)
	}

	const target = "SVar:Trig:DB$ Destroy | Defined$ TriggeredTargetLKICopy"
	self := "T:Mode$ DamageDone | ValidSource$ Card.Self | ValidTarget$ Creature | TriggerZones$ Battlefield | Execute$ Trig"

	// The read from a non-Self source has no object referent: the refusal
	// names the spelling.
	why, _, _ := ktqBuildSV(t, "T:Mode$ DamageDone | ValidSource$ Creature | ValidTarget$ Creature | TriggerZones$ Battlefield | Execute$ Trig", target)
	if !strings.Contains(why, `Defined$ "TriggeredTargetLKICopy" outside a damage trigger`) {
		t.Fatalf("non-Self source: refusal %q, want the triggered-target read refused by name", why)
	}

	// A body that reads the triggered source keeps today's plain trigger:
	// the Self/object gate does not widen every DamageDone line.
	why, ab, _ := ktqBuildSV(t, self, "SVar:Trig:DB$ Destroy | Defined$ TriggeredSource")
	if why != "" {
		t.Fatalf("TriggeredSource body: refusal %q", why)
	}
	if ab.Trig.Flags2&kop.Tf2PerReferent != 0 || ab.Trig.Flags2&kop.Tf2Batch != 0 {
		t.Fatalf("TriggeredSource body: per-referent flags %#x", ab.Trig.Flags2)
	}

	// A self-source object-referent body that reads another trigger-context
	// spelling refuses through the shared body-read gate.
	why, _, _ = ktqBuildSV(t, self,
		"SVar:Trig:DB$ Destroy | Defined$ TriggeredTargetLKICopy | SubAbility$ Other",
		"SVar:Other:DB$ Destroy | Defined$ TriggeredSource")
	if !strings.Contains(why, "body reads TriggeredSource") {
		t.Fatalf("mixed reads: refusal %q, want the second read refused by name", why)
	}
}
