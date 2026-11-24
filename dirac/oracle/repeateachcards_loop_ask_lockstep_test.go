package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// The plain RepeatEach-over-cards loop with a body that asks (dirac
// act_repeatcards.go repeatEachCardsFrame; kgen lower_repeatcardsloop.go):
// Cleansing's "for each land, destroy that land unless any player pays 1
// life" poses the unless payment each pass, with Remembered bound to that
// land, and the answer carries the loop on to the next land. Both engines must
// agree decision by decision through games where it resolves with lands out.
func krepeatAskDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Cleansing", Count: 10},
		{Name: "Plains", Count: 30},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// krepeatAskPolicy is castFirstPolicy, counting the unless-payment asks of
// Cleansing's loop it passes through.
func krepeatAskPolicy(asks *int) func(uint64) Policy {
	return func(seed uint64) Policy {
		base := castFirstPolicy(seed)
		return PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil && c.D.Kind == DModes {
				if pd := c.G.Pending(); pd != nil && pd.ResumeKind == "unless_pay" && chName(c.G.E.G, pd.Source) == "Cleansing" {
					*asks++
				}
			}
			return base.Pick(c)
		})
	}
}

func TestRepeatEachCardsLoopAskLockstep(t *testing.T) {
	t.Parallel()
	if _, err := cardset.CorpusDir(); err != nil {
		t.Skipf("SKIP: no corpus: %v", err)
	}
	asks := new(int)
	if div := lockstepRunPolicy(t, "repeateachcardsask", krepeatAskDecks(), 40, true, nil, nil, krepeatAskPolicy(asks)); div != 0 {
		t.Fatalf("%d seeds diverged", div)
	}
	if *asks == 0 {
		t.Fatal("Cleansing's unless payment was never asked; the lockstep proves nothing")
	}
	t.Logf("Cleansing unless-payment asks: %d", *asks)
}
