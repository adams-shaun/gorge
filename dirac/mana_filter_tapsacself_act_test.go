package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// tapSacSelfScen: p0 in main 1 with Lupinflower Village ({1}{W}, {T},
// Sacrifice) beside Three Tree Mascot ({1}: one mana of any color, the board's
// only white), Thornvault Forager and Darkstar Augur. The Village taps as cost
// and is sacrificed after the mana, so Forager's {G} pays the Mascot's {1}, the
// Mascot's {W} pays the {W} and a Swamp the generic (CR 601.2g-h, 605.3a).
func tapSacSelfScen(t *testing.T, swamps int) (*scen, Obj) {
	t.Helper()
	sc, id := newBLBScenTweak(t, []cardset.Entry{{Name: "Lupinflower Village", Count: 1},
		{Name: "Three Tree Mascot", Count: 1}, {Name: "Thornvault Forager", Count: 1}, {Name: "Darkstar Augur", Count: 1}, {Name: "Swamp", Count: 7}, {Name: "Plains", Count: 1}}, nil)
	v := sc.perm(id["Lupinflower Village"], 0)
	sc.perm(id["Three Tree Mascot"], 0)
	sc.perm(id["Thornvault Forager"], 0)
	sc.perm(id["Darkstar Augur"], 0)
	for i := 0; i < swamps; i++ {
		sc.perm(id["Swamp"], 0)
	}
	return sc, v
}

func activateOptions(sc *scen, o Obj) (n int) {
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(o)) {
			n++
		}
	}
	return n
}

// TestTapSacSelfActivationFundedByFilter: a "{T}, Sacrifice this" activation
// whose {W} only a filter makes is offered once the Forager's {G} pays the
// filter's fee and a Swamp the generic, and not without that Swamp.
func TestTapSacSelfActivationFundedByFilter(t *testing.T) {
	for _, c := range []struct{ swamps, want int }{{0, 0}, {1, 1}, {2, 1}} {
		sc, v := tapSacSelfScen(t, c.swamps)
		if got := activateOptions(sc, v); got != c.want {
			t.Errorf("%d Swamps: %d Village activations offered, want %d", c.swamps, got, c.want)
		}
	}
}
