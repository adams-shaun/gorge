package kgen

import (
	"strings"
	"testing"
)

// playerStaticCarriers are the `Affected$ You` player statics the kernel
// models: Defensive Formation's "You assign combat damage of each creature
// attacking you." (engine/rules/playerkeywords.go). It lowers to one
// StatDefenderAssigns static.
var playerStaticCarriers = []string{"Defensive Formation"}

// playerStaticRefusals are the player-keyword carriers that stay refused:
// the four gorge has no reader for (lowering them would copy a gorge gap)
// and K'rrik's PayLifeInsteadOf:B, which gorge reads
// (engine/rules/mana.go PayLifeInsteadOfB) but the kernel does not model yet.
var playerStaticRefusals = []struct{ card, want string }{
	{"K'rrik, Son of Yawgmoth", "player static"},
	{"Ballot Broker", "player static"},
	{"Brago's Representative", "player static"},
	{"Winter Orb", `keyword "UntapAdjust:Land:1"`},
	{"Thick-Skinned Goblin", "player static"},
}

func TestStaticZonesPlayerStaticForms(t *testing.T) {
	for _, name := range playerStaticCarriers {
		c := lowerNamedCard(t, name)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", name, c.Refusals)
		}
		n := 0
		for _, st := range c.Statics {
			if st.Kind == "DefenderAssigns" {
				n++
			}
		}
		if n != 1 {
			t.Errorf("%s lowers %d DefenderAssigns statics, want 1", name, n)
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
