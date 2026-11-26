package kgen

import (
	"strings"
	"testing"
)

// playerStaticCarriers are the `Affected$ You` player statics the kernel
// models: Defensive Formation's "You assign combat damage of each creature
// attacking you." (engine/rules/playerkeywords.go) and K'rrik, Son of
// Yawgmoth's PayLifeInsteadOf:B grant (engine/rules/mana.go
// PayLifeInsteadOfB). Each lowers to one static of its kind.
var playerStaticCarriers = []struct{ card, kind string }{
	{"Defensive Formation", "DefenderAssigns"},
	{"K'rrik, Son of Yawgmoth", "PayLifeInsteadOfB"},
}

// playerStaticRefusals are the player-keyword carriers that stay refused:
// the four gorge has no reader for (lowering them would copy a gorge gap).
var playerStaticRefusals = []struct{ card, want string }{
	{"Ballot Broker", "player static"},
	{"Brago's Representative", "player static"},
	{"Winter Orb", `keyword "UntapAdjust:Land:1"`},
	{"Thick-Skinned Goblin", "player static"},
}

func TestStaticZonesPlayerStaticForms(t *testing.T) {
	for _, cr := range playerStaticCarriers {
		c := lowerNamedCard(t, cr.card)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", cr.card, c.Refusals)
		}
		n := 0
		for _, st := range c.Statics {
			if st.Kind == cr.kind {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s lowers %d %s statics, want 1", cr.card, n, cr.kind)
		}
	}
	for _, r := range playerStaticRefusals {
		c := lowerNamedCard(t, r.card)
		hit := false
		for _, rf := range c.Refusals {
			hit = hit || strings.Contains(rf.Reason, r.want)
		}
		if !hit {
			t.Errorf("%s refusals %v: none carries %q", r.card, c.Refusals, r.want)
		}
	}
}
