package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/engine/cards"
)

// TestDifferentCounterKindsLowers: the creatures-you-control selector of
// Count$DifferentCounterKinds_ lowers (Hundred-Battle Veteran's self-pump
// gate), the other two selectors gorge reads refuse by name, and an unknown
// selector refuses.
func TestDifferentCounterKindsLowers(t *testing.T) {
	lc := &lowerCtx{face: &cards.Face{SVars: map[string]string{}}}
	v, err := lc.svarValue("Count$DifferentCounterKinds_Creature.YouCtrl", 0)
	if err != nil || v.Kind != vCounterKinds || v.F == nil {
		t.Fatalf("Creature.YouCtrl: got %s, %v; want CounterKinds over a filter", v, err)
	}
	for _, spec := range []string{"Card.Self", "Permanent.YouCtrl", "Land.YouCtrl"} {
		v, err := lc.svarValue("Count$DifferentCounterKinds_"+spec, 0)
		if err == nil {
			t.Errorf("%s lowered to %s; want a refusal", spec, v)
		} else if !strings.Contains(err.Error(), "DifferentCounterKinds_"+spec) {
			t.Errorf("%s: refusal %q does not name the selector", spec, err)
		}
	}
	cs := lowerNamed(t, []string{"Hundred-Battle Veteran"})
	if len(cs) != 1 || cs[0].Name != "Hundred-Battle Veteran" {
		t.Fatalf("lowered %d cards; want Hundred-Battle Veteran", len(cs))
	}
	for _, r := range cs[0].Refusals {
		t.Errorf("Hundred-Battle Veteran refused: %s", r.Reason)
	}
}
