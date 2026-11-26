package kgen

import (
	"strings"
	"testing"
)

// effectSelfRemCarriers are until-end-of-turn Effects with RememberObjects$
// Self over a SpellCast or DamageDone trigger whose body reads the source
// back as Defined$ Remembered (emblemSelfRemember): the body's PutCounter
// lowers over rEmblemSource.
var effectSelfRemCarriers = []string{"Mountain Titan", "Dovin, Grand Arbiter"}

func TestRememberedEffectSelfLowers(t *testing.T) {
	for _, name := range effectSelfRemCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		found := false
		for _, ab := range c.Abilities {
			for _, in := range ab.Prog {
				if in.Op == "PutCounter" && in.Obj.Kind == rEmblemSource {
					found = true
				}
			}
		}
		if !found {
			t.Errorf("%s: no PutCounter over EmblemSource in its emblem trigger", name)
		}
	}
}

// TestRememberedSelfOtherModesRefuse: RememberObjects$ Self beside a trigger
// mode outside SpellCast/DamageDone (Massacre Girl's and Electric Seaweed's
// ChangesZone, whose filters read !IsRemembered) stays refused.
func TestRememberedSelfOtherModesRefuse(t *testing.T) {
	for _, name := range []string{"Massacre Girl", "Electric Seaweed"} {
		c := lowerNamedCard(t, name)
		hit := false
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, `RememberObjects$ "Self"`) {
				hit = true
			}
		}
		if !hit {
			t.Errorf("%s refusals = %v, want the RememberObjects$ Self refusal", name, c.Refusals)
		}
	}
}
