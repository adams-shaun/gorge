package kgen

import (
	"strings"
	"testing"
)

// TestImprintMemorySlice: the gap-imprint-memory slice this change lowers.
// Dazzling Reflection's Effect imprints the parent's target and keeps a
// source-keyed shield on it (lower_effectimprintshield.go): one Ext
// PreventNext keyed on the target creature, no refusal.
func TestImprintMemorySlice(t *testing.T) {
	for _, name := range []string{"Dazzling Reflection"} {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		found := false
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				found = found || in.Op == "PreventNext" && in.Obj.Kind == rTarget && strings.Contains(in.Note, "imprinted")
			}
		}
		if !found {
			t.Errorf("%s: no PreventNext on the target", name)
		}
	}
}

// An imprint Effect with a trigger beyond the leave-exile one keeps its
// refusal: Fire Giant's Fury's damage trigger reads the imprinted Giant.
func TestImprintShieldOtherTriggerRefuses(t *testing.T) {
	if c := lowerNamedCard(t, "Fire Giant's Fury"); len(c.Refusals) == 0 {
		t.Errorf("Fire Giant's Fury lowers; want its imprint-trigger refusal kept")
	}
}
