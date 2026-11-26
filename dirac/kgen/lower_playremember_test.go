package kgen

import (
	"strings"
	"testing"
)

// RememberPlayed$ True on api:Play lowers to OpCastFree's Remember flag (C bit
// 13, dirac/cast_playremember.go). The carriers below keep their other
// blockers; the class named "Play RememberPlayed" is gone from each, and any
// value but True refuses by name.
var playRememberCarriers = []string{
	"Kaervek, the Punisher", "Kellan, the Kid", "Chandra, Torch of Defiance", "Planetarium of Wan Shi Tong",
	"Kylox's Voltstrider", "Cmdr. Troi, Outgoing Empath", "Baral and Kari Zev", "Itazura, Lingering Wick", "Helmut Zemo, Mastermind",
	"Conduit of Worlds",
}

func TestPlayRememberPlayedShapes(t *testing.T) {
	seen := 0
	for _, c := range lowerNamed(t, playRememberCarriers) {
		seen++
		for _, r := range c.Refusals {
			if strings.Contains(r.Reason, "RememberPlayed") {
				t.Errorf("%s still refuses on RememberPlayed: %s", c.Name, r.Reason)
			}
		}
		if c.Name != "Kellan, the Kid" {
			continue
		}
		if len(c.Refusals) != 0 {
			t.Errorf("Kellan, the Kid refusals = %v, want none", c.Refusals)
		}
		found := false
		for _, a := range c.Abilities {
			for _, in := range a.Prog {
				if in.Op == "CastFree" && strings.Contains(in.CS, "Remember") {
					found = true
				}
			}
		}
		if !found {
			t.Error("Kellan, the Kid: no CastFree carries the Remember flag")
		}
	}
	if seen != len(playRememberCarriers) {
		t.Fatalf("lowered %d carriers, want %d", seen, len(playRememberCarriers))
	}
}

func TestPlayRememberPlayedNonTrueRefuses(t *testing.T) {
	src := strings.Join([]string{
		"Name:Probe",
		"ManaCost:1 R",
		"Types:Creature Human",
		"PT:1/1",
		"A:AB$ Play | Cost$ T | Valid$ Card.YouOwn | ValidZone$ Hand | ValidSA$ Spell | WithoutManaCost$ True | Amount$ 1 | Optional$ True | RememberPlayed$ False | SpellDescription$ Probe.",
	}, "\n")
	if got := abReasons(abLower(t, src, false)); !strings.Contains(got, `Play RememberPlayed "False"`) {
		t.Fatalf("refusals = %q, want the named RememberPlayed refusal", got)
	}
}
