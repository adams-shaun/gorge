package rules

import (
	"testing"

	"github.com/adams-shaun/gorge/effects"
	"github.com/adams-shaun/gorge/internal/testutil"
)

// TestAlternativeCostCorpusCards proves the keyword parser and rules registry
// consume the actual Forge scripts, rather than look-alike hand fixtures.
func TestAlternativeCostCorpusCards(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, tc := range []struct {
		name string
		prim string
	}{
		{"Constant Mists", "kw:Buyback"},
		{"Dizzy Spell", "kw:Transmute"},
		{"Profane Tutor", "kw:Suspend"},
		{"Crowd's Favor", "kw:Convoke"},
		{"Wild Ride", "kw:Harmonize"},
		{"Ziatora's Proving Ground", "kw:Cycling"},
	} {
		_, ok := reg.Lookup(tc.name)
		if !ok {
			t.Fatalf("corpus missing %s", tc.name)
		}
		if !effects.Supported()[tc.prim] {
			t.Errorf("%s not registered for %s", tc.prim, tc.name)
		}
	}

	for _, name := range []string{"Dizzy Spell", "Ziatora's Proving Ground"} {
		c, _ := reg.Lookup(name)
		found := false
		for _, ab := range c.Faces[0].Abilities {
			if ab.Params["ActivationZone"] == "Hand" {
				found = true
			}
		}
		if !found {
			t.Errorf("%s did not expand to a hand activation", name)
		}
	}
}

func TestCastWithFlashCorpusStaticsAreRegistered(t *testing.T) {
	reg := testutil.CorpusRegistry(t)
	for _, name := range []string{"Skittering Cicada", "Vedalken Orrery"} {
		c, ok := reg.Lookup(name)
		if !ok || len(c.Faces[0].Statics) == 0 || c.Faces[0].Statics[0].Mode != "CastWithFlash" {
			t.Errorf("%s has no CastWithFlash static in corpus IR", name)
		}
	}
	if !effects.Supported()["stat:CastWithFlash"] {
		t.Fatal("stat:CastWithFlash is not registered")
	}
}
