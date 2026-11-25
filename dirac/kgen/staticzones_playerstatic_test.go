package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// playerStaticCarriers are the two `Affected$ You` player statics gorge
// reads and the kernel now models: K'rrik's PayLifeInsteadOf:B
// (engine/rules/mana.go PayLifeInsteadOfB) and Defensive Formation's "You
// assign combat damage of each creature attacking you."
// (engine/rules/playerkeywords.go). Each lowers to one StatPlayerKeyword
// static whose Amount names the keyword.
var playerStaticCarriers = []struct {
	card   string
	amount int
}{
	{"K'rrik, Son of Yawgmoth", kop.PlKwPayLifeInsteadOfB},
	{"Defensive Formation", kop.PlKwDefenderAssigns},
}

// playerStaticRefusals are the player-keyword carriers gorge has no reader
// for: lowering them would copy a gorge gap, so each keeps the standing
// "player static" refusal.
var playerStaticRefusals = []struct{ card, want string }{
	{"Ballot Broker", "player static"},
	{"Brago's Representative", "player static"},
	{"Winter Orb", `keyword "UntapAdjust:Land:1"`},
	{"Thick-Skinned Goblin", "player static"},
}

func TestStaticZonesPlayerStaticForms(t *testing.T) {
	for _, w := range playerStaticCarriers {
		c := lowerNamedCard(t, w.card)
		if len(c.Refusals) != 0 {
			t.Errorf("%s refusals = %v, want none", w.card, c.Refusals)
		}
		n := 0
		for _, st := range c.Statics {
			if st.Kind == "PlayerKeyword" {
				n++
				if !st.Amount.isConst() || st.Amount.N != w.amount {
					t.Errorf("%s PlayerKeyword amount = %v, want %d", w.card, st.Amount, w.amount)
				}
			}
		}
		if n != 1 {
			t.Errorf("%s lowers %d PlayerKeyword statics, want 1", w.card, n)
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
