package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// An "EACH A & B" pick takes one card per listed type: Lotuslight Dancers
// searches for a black, a green and a blue card, so a multicolour card fills
// one slot only; Michelangelo may put a creature and a land from hand, and a
// creature land (Dryad Arbor) is offered as a creature, never a second pick.
// The kernel's candidate set and pick count follow gorge's per-type groups.

var eachpickDecks = [3][]cardset.Entry{
	{
		{Name: "Lotuslight Dancers", Count: 6}, {Name: "Dimir Guildmage", Count: 3},
		{Name: "Golgari Guildmage", Count: 3}, {Name: "Simic Sky Swallower", Count: 3},
		{Name: "Vampire Nighthawk", Count: 3}, {Name: "Grizzly Bears", Count: 2},
		{Name: "Swamp", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Michelangelo, Improviser", Count: 6}, {Name: "Dryad Arbor", Count: 4},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Wall of Omens", Count: 4},
		{Name: "Forest", Count: 14}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Aang's Journey", Count: 12}, {Name: "Honden of Cleansing Fire", Count: 6},
		{Name: "Plains", Count: 7}, {Name: "Mountain", Count: 7}, {Name: "Forest", Count: 8},
	},
}

// eachpickCases names the two pick shapes: the library search and the hand move.
var eachpickCases = []struct {
	name string
	deck int
}{{"search", 0}, {"handmove", 1}, {"kicked", 2}}

// eachpickKickedPolicy casts, taking a kicked cast whenever one is offered.
func eachpickKickedPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		kicked := false
		for _, a := range c.D.Options {
			k := a.Kind()
			if k != ActCast && k != ActCastFace && k != ActActivate && k != ActPlayLand {
				continue
			}
			if k == ActCast && c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok && o.Mode == "kicked" {
					if !kicked {
						buf, kicked = buf[:0], true
					}
					buf = append(buf, a)
					continue
				}
			}
			if !kicked {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

func TestEachPickPlaysInLockstep(t *testing.T) {
	for _, c := range eachpickCases {
		d := eachpickDecks[c.deck]
		if n := lockstepRunPolicy(t, "eachpick", [2][]cardset.Entry{d, d}, 150, true, nil, nil, eachpickKickedPolicy); n != 0 {
			t.Fatalf("%s: %d divergent games", c.name, n)
		}
	}
}

// TestEachPickNeedsItsGroups clears the EACH marks: the kernel then picks
// from the union and diverges from gorge, in both shapes.
func TestEachPickNeedsItsGroups(t *testing.T) {
	for _, c := range eachpickCases {
		d := eachpickDecks[c.deck]
		n := lockstepRunPolicy(t, "eachpick", [2][]cardset.Entry{d, d}, 150, false, func(tb *kop.Table) {
			for i := range tb.Filters {
				tb.Filters[i].Each = false
			}
		}, nil, eachpickKickedPolicy)
		if n == 0 {
			t.Errorf("%s: no game diverged without the EACH groups: the pool does not reach the pick", c.name)
		}
	}
}
