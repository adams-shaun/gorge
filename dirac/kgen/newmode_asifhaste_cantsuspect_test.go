package kgen

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestNewModeAsIfHasteCantSuspect: Shang-Chi, Master of Kung Fu's
// ActivateAbilityAsIfHaste lowers to one StatAsIfHaste static and Airtight
// Alibi's CantBeSuspected to one StatCantBeSuspected static, each carrying
// its ValidCard$ filter, and neither card refuses anything else.
func TestNewModeAsIfHasteCantSuspect(t *testing.T) {
	want := map[string]kop.StaticKind{
		"Shang-Chi, Master of Kung Fu": kop.StatAsIfHaste,
		"Airtight Alibi":               kop.StatCantBeSuspected,
	}
	lw := lowerNamedPool(t, []string{"Shang-Chi, Master of Kung Fu", "Airtight Alibi"})
	all, bs, _, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int{}
	for i, c := range all {
		if r := lowerRefusals(c); len(r) > 0 {
			t.Errorf("%s: refused: %v", c.Name, r)
		}
		sk, ok := want[c.Name]
		if !ok {
			continue
		}
		for _, a := range bs[i].def.Abilities {
			if a.Kind == kop.AbStatic && a.Static.Kind == sk {
				got[c.Name]++
				if a.Static.Filter == 0 {
					t.Errorf("%s: static %d has no filter", c.Name, sk)
				}
			}
		}
	}
	for name := range want {
		if got[name] != 1 {
			t.Errorf("%s: statics of its kind = %d, want 1", name, got[name])
		}
	}
	for _, c := range []struct{ name, line string }{
		{"unscoped AsIfHaste refuses", "S:Mode$ ActivateAbilityAsIfHaste | Description$ x"},
		{"unscoped CantBeSuspected refuses", "S:Mode$ CantBeSuspected | Description$ x"},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := kcantselfRefusals(t, c.line); got == "" {
				t.Error("want a refusal, got none")
			}
		})
	}
}
