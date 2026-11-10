//go:build dirac_mp

package canon

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
)

// TestAttackPlayerCanonAtNSeats: an attack at a player names the defending
// seat the kernel's option carries (gorge prints attack(#n,Pq)), not the two-seat
// "the other player", at three and four seats and from every attacking seat.
func TestAttackPlayerCanonAtNSeats(t *testing.T) {
	const atk = Ref(7)
	for _, seats := range []uint8{3, 4} {
		for active := uint8(0); active < seats; active++ {
			s := new(dirac.State)
			d := &dirac.Decision{Kind: dirac.DkAttackers, Player: active}
			tr := &Translator{S: s, D: d}
			for def := uint8(0); def < seats; def++ {
				if def == active {
					continue
				}
				got := tr.Action(dirac.MakeAction(dirac.AkAttack, uint8(atk), dirac.PlayerCode(def), 0))
				want := Pack(ActAttack, atk, RefPlayer+Ref(def), 0)
				if got != want {
					t.Fatalf("%d seats, p%d attacking p%d: %v, want %v", seats, active, def, got, want)
				}
			}
		}
	}
}
