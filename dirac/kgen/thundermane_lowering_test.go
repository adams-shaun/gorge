package kgen

import (
	"strings"
	"testing"
)

// TestThundermaneLowering: Thundermane Dragon's library-top permission lowers
// with its ValidAfterStack$ spell qualifier (Spell.Creature+powerGE4) folded
// into the filter: creature, nonland, owned, power >= 4. Its static SpellCast
// haste rider lowers too, as a static trigger whose timed Animate is marked
// spellCarry: gorge carries the grant onto the permanent the spell becomes
// (CR 400.7a; engine/effects/combatfx.go ExileOnMovedStackCarry), so the
// kernel mirrors it as a record stamped on the spell entry and applied as the
// permanent enters (staticSpellCastBody in lower_statictrigger_spellcast.go).
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
	if len(c.Refusals) != 0 {
		t.Errorf("refusals %v, want none", c.Refusals)
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
