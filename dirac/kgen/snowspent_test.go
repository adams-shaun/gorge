package kgen

import (
	"strings"
	"testing"
)

// TestSnowSpentLowers: Count$CastTotalManaSpent Snow lowers to the spend read
// and marks the card SpentSnow (the kernel counts the snow units the cast's
// pool payment took); a face that reads the plain total beside it refuses.
func TestSnowSpentLowers(t *testing.T) {
	for _, name := range []string{"Berg Strider", "Graven Lore", "Tundra Fumarole"} {
		lc := lowerCorpusCard(t, name)
		if r := probeRefusals(lc); r != "" {
			t.Errorf("%s refused: %s", name, r)
		}
		if !lc.SpentSnow {
			t.Errorf("%s: SpentSnow not set", name)
		}
		if !lc.SpendRead {
			t.Errorf("%s: SpendRead not set (the planner must decline as gorge's does)", name)
		}
	}
	const both = `Name:Both Probe
ManaCost:2 U
Types:Sorcery
A:SP$ Draw | NumCards$ SnowUnits | SubAbility$ DBLose
SVar:DBLose:DB$ LoseLife | LifeAmount$ AllUnits
SVar:SnowUnits:Count$CastTotalManaSpent Snow
SVar:AllUnits:Count$CastTotalManaSpent
Oracle:probe
`
	lc := lowerProbe(t, both, "Both Probe", "")
	if r := probeRefusals(lc); !strings.Contains(r, "Count$CastTotalManaSpent read in more than one form") {
		t.Errorf("probe refusals %q, want the more-than-one-form refusal", r)
	}
}
