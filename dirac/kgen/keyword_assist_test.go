package kgen

import (
	"strings"
	"testing"
)

// assistCarriers print K:Assist (CR 702.132). gorge's engine has no Assist
// (no cast-path payment by another player; engine/rules/testdata/
// granted-keyword-gaps.txt pins the gap and 702-keyword-abilities.md marks
// 702.132 "kw:Assist unregistered"), so lowering the keyword would make dirac
// play the card as a plain spell exactly like gorge and neither would ask the
// assisting seat. Every carrier therefore stays refused by name until gorge
// reads the keyword.
var assistCarriers = []string{
	"Bloodborn Scoundrels", "Bring Down", "Charging Binox", "Dwarven Lightsmith",
	"Fan Favorite", "Game Plan", "Gang Up", "Huddle Up", "Lava-Field Overlord",
	"Magma Hellion", "Out of Bounds", "Play of the Game", "Skystreamer",
	"Spellweaver Duo", "The Crowd Goes Wild", "Vampire Charmseeker",
}

func TestKeywordAssist(t *testing.T) {
	seen := map[string]bool{}
	for _, c := range lowerNamed(t, assistCarriers) {
		seen[c.Name] = true
		var all []string
		for _, r := range c.Refusals {
			all = append(all, r.Reason)
		}
		got := strings.Join(all, "; ")
		if !strings.Contains(got, `keyword "Assist"`) {
			t.Errorf("%s: refusals %q, want the keyword \"Assist\" refusal", c.Name, got)
		}
	}
	for _, n := range assistCarriers {
		if !seen[n] {
			t.Errorf("precondition: %s not lowered (corpus missing?)", n)
		}
	}
}
