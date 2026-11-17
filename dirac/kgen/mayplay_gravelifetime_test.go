package kgen

import (
	"fmt"
	"testing"
)

// TestMayPlayGraveLifetime: an unlimited until-end-of-turn Effect grant over a
// remembered graveyard card lowers to the re-arming WhileGraveForget record
// only with ForgetOnMoved$ Exile (Skyclave Shade). gorge's effectMoveSweep
// prunes a grant with no ForgetOnMoved$ (Reezug, the Bonecobbler) the moment
// the card leaves the graveyard (CR 400.7), so it, a limited grant, an
// ExileOnMoved grant and any other ForgetOnMoved$ keep the one-shot EOT dur.
func TestMayPlayGraveLifetime(t *testing.T) {
	const src = "Name:Probe\nManaCost:1 B\nTypes:Creature Zombie\nPT:1/1\n" +
		"A:AB$ Effect | Cost$ T | ValidTgts$ Card.YouOwn | TgtZone$ Graveyard | RememberObjects$ Targeted | StaticAbilities$ Play%s | SpellDescription$ Probe.\n" +
		"SVar:Play:Mode$ Continuous | MayPlay$ True | Affected$ Card.IsRemembered | AffectedZone$ Graveyard%s | Description$ Probe.\n"
	for _, tc := range []struct{ name, eff, st, want string }{
		{"forget exile", " | ForgetOnMoved$ Exile", "", "WhileGraveForget"},
		{"no forget (Reezug)", "", "", "EOT"},
		{"forget graveyard", " | ForgetOnMoved$ Graveyard", "", "EOT"},
		{"exile on moved", " | ExileOnMoved$ Graveyard", "", "EOT"},
		{"limited", " | ForgetOnMoved$ Exile", " | MayPlayLimit$ 1", "EOT"},
	} {
		c := mzLower(t, fmt.Sprintf(src, tc.eff, tc.st))
		if r := mzReasons(c); r != "" {
			t.Errorf("%s: refused: %s", tc.name, r)
			continue
		}
		got := mzMayPlays(c)
		if len(got) != 1 {
			t.Errorf("%s: %d MayPlay instructions, want 1", tc.name, len(got))
			continue
		}
		if got[0].CS != tc.want {
			t.Errorf("%s: lifetime %q, want %q", tc.name, got[0].CS, tc.want)
		}
	}
}
