package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// Into the Wilds (CR 603.5, 701.20a, 608.2d): the upkeep trigger looks at the
// top card of the library and, if it is a land, the controller may put it onto
// the battlefield. gorge poses one yes/no over the Defined$ Remembered fetch
// (moveDefinedLibraryObjects' defined_library_optional) and never when the top
// card is not a land or the library is empty; a decline leaves the card on top. The expected
// answer at every decision, and the library order after each upkeep, is
// gorge's.
var kintoTheWildsDecks = [2][]cardset.Entry{
	{
		{Name: "Into the Wilds", Count: 12}, {Name: "Forest", Count: 10}, {Name: "Island", Count: 8},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6},
		{Name: "Forest", Count: 13}, {Name: "Island", Count: 13},
	},
}

// kintoTheWildsPolicy casts first and answers the may-put ask yes two times in
// three, counting both answers; every other decision is castFirstPolicy's.
func kintoTheWildsPolicy(yes, no *int) func(uint64) Policy {
	yesK, _ := chooseCode("yes")
	noK, _ := chooseCode("no")
	return func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.D.Kind == DChoose && c.G != nil && len(c.D.Options) == 2 &&
				c.D.Options[0].Kind() == ActChoose+CanonKind(yesK) && c.D.Options[1].Kind() == ActChoose+CanonKind(noK) {
				if d := c.G.Pending(); d != nil && d.ResumeKind == "defined_library_optional" {
					if c.Seq%3 != 0 {
						*yes++
						return c.D.Options[0]
					}
					*no++
					return c.D.Options[1]
				}
			}
			return base.Pick(c)
		})
	}
}

func TestIntoTheWildsLockstep(t *testing.T) {
	t.Parallel()
	yes, no := 0, 0
	div := lockstepRunPolicy(t, "intothewilds", kintoTheWildsDecks, 200, true, nil, nil, kintoTheWildsPolicy(&yes, &no))
	t.Logf("%d divergent games; may-put asks answered yes %d, no %d", div, yes, no)
	if yes < 30 {
		t.Errorf("%d may-put asks answered yes, want at least 30", yes)
	}
	if no < 15 {
		t.Errorf("%d may-put asks answered no, want at least 15", no)
	}
}
