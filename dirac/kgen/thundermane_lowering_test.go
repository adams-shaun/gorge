package kgen

import (
	"strings"
	"testing"
)

// TestThundermaneLowering: Thundermane Dragon's library-top permission lowers
// with its ValidAfterStack$ spell qualifier (Spell.Creature+powerGE4) folded
// into the filter: creature, nonland, owned, power >= 4. Its static SpellCast
// haste rider is the one node still refused: gorge ends an Animate grant on a
// spell when the spell leaves the stack (engine/effects/combatfx.go,
// ExileOnMoved$ Stack), so the cast creature never has the haste the card
// grants and there is no gorge behaviour for the kernel to mirror.
func TestThundermaneLowering(t *testing.T) {
	cs := lowerNamed(t, []string{"Thundermane Dragon"})
	if len(cs) != 1 {
		t.Fatalf("%d cards lowered, want 1", len(cs))
	}
	c := cs[0]
	var dump strings.Builder
	dumpLIR(&dump, c)
	if !strings.Contains(dump.String(), "static MayPlayFrom F={types=0x2 not=0x1 owner=1 cmp(1 1 4  )}") {
		t.Errorf("the library permission lacks the folded qualifier:\n%s", dump.String())
	}
	if len(c.Refusals) != 1 || c.Refusals[0].Node != "T[0] Mode=SpellCast" {
		t.Errorf("refusals %v, want only the static SpellCast haste rider", c.Refusals)
	}
}

// TestLibraryAfterStackFoldRefusals: the fold needs every alternative to
// exclude lands (gorge never asks a land the spell qualifier), so Glarb's
// Land.TopLibrary alternative stays refused.
func TestLibraryAfterStackFoldRefusals(t *testing.T) {
	for _, c := range lowerNamed(t, []string{"Glarb, Calamity's Augur"}) {
		found := false
		for _, r := range c.Refusals {
			found = found || strings.Contains(r.Reason, "ValidAfterStack$")
		}
		if !found {
			t.Errorf("%s: no ValidAfterStack$ refusal: %v", c.Name, c.Refusals)
		}
	}
}
