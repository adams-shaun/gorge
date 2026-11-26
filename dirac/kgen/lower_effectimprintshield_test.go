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

// TestImprintHandPickRepeat: Eureka and Hypergenesis lower whole. Each pass
// of the Repeat clears the imprinted set, the hand pick onto the battlefield
// imprints (ChooseMove CS Imprint, kop.SrImprint), and the gate counts it.
func TestImprintHandPickRepeat(t *testing.T) {
	for _, name := range []string{"Eureka", "Hypergenesis"} {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		clears, pick := 0, false
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				if in.Op == "ClearImprinted" {
					clears++
				}
				pick = pick || in.Op == "ChooseMove" && strings.Contains(in.CS, "from=Hand") && strings.Contains(in.CS, "|Imprint")
			}
		}
		if clears != 2 || !pick {
			t.Errorf("%s: %d ClearImprinted (want 2), imprinting hand pick %v", name, clears, pick)
		}
	}
}

// TestImprintMemoryShapes: the other gap-imprint-memory shapes lower whole.
// Parallax Nexus's hand pick into exile records on the persist store, Phelia
// brackets her Return with ImprintRef/ForgetImprinted (OpReturnEx has no
// flag left), and Eye of Singularity's imprint loop folds into one
// sharesNameWith Valid sweep.
func TestImprintMemoryShapes(t *testing.T) {
	want := map[string]string{
		"Parallax Nexus":             "ChooseMove",
		"Phelia, Exuberant Shepherd": "ImprintRef",
		"Eye of Singularity":         "DestroyAll",
	}
	for name, op := range want {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
			continue
		}
		found := false
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				found = found || in.Op == op
			}
		}
		if !found {
			t.Errorf("%s: no %s", name, op)
		}
	}
}
