package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestCantLoseLifeReason: a can't-lose replacement that names only the loss
// for 0 or less life (ValidLoseReason$ LifeReachedZero) stops that state-based
// loss and no other (CR 704.5a, 704.5c, 704.5b): the controller at 0 life
// survives the check, while ten poison counters still lose; an unreasoned
// replacement stops both.
func TestCantLoseLifeReason(t *testing.T) {
	for _, tc := range []struct {
		name   string
		c      int16
		poison bool
		lost   bool
	}{
		{"life, reasoned", kop.CantLoseLife, false, false},
		{"poison, reasoned", kop.CantLoseLife, true, true},
		{"life, any reason", 0, false, false},
		{"poison, any reason", 0, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sc, a, _ := infectScen(t, true, 0)
			c := sc.s.objs[a].card
			sc.k.t.Cards[c].Abilities = append(sc.k.t.Cards[c].Abilities,
				kop.Ability{Kind: kop.AbReplacement, Repl: kop.Repl{Kind: kop.ReplCantLoseGame, C: tc.c}})
			sc.s.statM = sc.s.statM.with(sc.slot(a))
			if tc.poison {
				sc.s.addPoison(0, 10)
			} else {
				sc.s.pl[0].life = 0
			}
			sc.s.sba(sc.k)
			if got := sc.s.pl[0].lost; got != tc.lost {
				t.Fatalf("lost = %v, want %v", got, tc.lost)
			}
		})
	}
}
