package kgen

import (
	"strings"
	"testing"
)

// TestAnimateKernelSurfaceLeftovers: the two Animate kernel-surface carriers
// that kept a second blocker after TestAnimateShapesKernelSurfacesSlice.
// Realmbreaker, the Invasion Tree's ChangeType$ Land.TargetedPlayerCtrl
// graveyard return lowers onto the graveyard pick with the targeted player's
// ownership (a graveyard card's controller is its owner) and RememberChanged$
// for the chained Animate. Koh, the Face Stealer's GainsTriggerAbsOf$ is
// refused by name: the kernel has no gained-trigger walk (a measured
// StallGainsOf on the Koh lockstep pool), not as an unread parameter.
func TestAnimateKernelSurfaceLeftovers(t *testing.T) {
	rb := lowerNamedCard(t, "Realmbreaker, the Invasion Tree")
	if len(rb.Refusals) != 0 {
		t.Errorf("Realmbreaker refusals = %v, want none", rb.Refusals)
	}
	var sb strings.Builder
	dumpLIR(&sb, rb)
	got := sb.String()
	var pick string
	for _, l := range strings.Split(got, "\n") {
		if strings.Contains(l, "ChooseMove") {
			pick = l
		}
	}
	for _, want := range []string{"from=Graveyard", "Tapped", "Remember", "UnderYourControl", "types=0x1 owner=4"} {
		if !strings.Contains(pick, want) {
			t.Errorf("Realmbreaker's graveyard pick lacks %q:\n%s", want, got)
		}
	}
	if !strings.Contains(got, "Override obj=Remembered") || !strings.Contains(got, "exileonleave") {
		t.Errorf("Realmbreaker's Animate does not read the returned land:\n%s", got)
	}

	koh := lowerNamedCard(t, "Koh, the Face Stealer")
	found := false
	for _, r := range koh.Refusals {
		if strings.Contains(r.Reason, "unread params") && strings.Contains(r.Reason, "GainsTriggerAbsOf") {
			t.Errorf("Koh refuses GainsTriggerAbsOf$ as an unread parameter: %v", r)
		}
		found = found || strings.Contains(r.Reason, "no gained-trigger walk")
	}
	if !found || len(koh.Refusals) != 1 {
		t.Errorf("Koh refusals = %v, want the one named gained-trigger refusal", koh.Refusals)
	}
}
