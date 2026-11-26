package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestSharedTargetSecondTriggerStays: two Harness the Storm triggers target
// the same graveyard card. The first trigger's paid Play begins the cast, and
// the one Mountain the first spell left cannot pay for it, so the cast is
// reversed (CR 733.1). A reversed cast puts the card back as the same object,
// so the second trigger's target is still legal when it resolves (CR 608.2b):
// it stays on the stack and poses its own Play ask, as gorge does. The
// kernel's push bumped the card's version and the reversal kept the bump, so
// the second trigger fizzled without an ask.
func TestSharedTargetSecondTriggerStays(t *testing.T) {
	set := cardsettest.Set(t, "playpopcount")
	deck := []cardset.Entry{{Name: "Harness the Storm", Count: 2}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 34}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, id["Mountain"], Config{NoElide: true})
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Harness the Storm"], 0)
	sc.perm(id["Harness the Storm"], 0)
	g := sc.grave(id["Shock"], 0, 1)[0]
	sc.castCard(t, sc.hand(id["Shock"], 0))

	asks := 0
	for i := 0; i < 80 && asks < 2; i++ {
		d := sc.s.Step()
		if d.Kind == DkNone {
			break
		}
		a := d.Options[0]
		switch {
		case d.Kind == DkChoose && d.Sub == ChGenCastFree:
			asks++
			if d.N < 2 {
				t.Fatalf("play ask %d offers %d options, want the card and done", asks, d.N)
			}
			if z := sc.s.objs[g].zone; z != kop.ZoneGraveyard {
				t.Fatalf("play ask %d: the shared target is in zone %v, want its graveyard", asks, z)
			}
			a = d.Options[1]
		case d.Kind == DkPriority:
			a = MakeAction(AkPass, 0, 0, 0)
		}
		sc.s.Apply(a)
	}
	if asks != 2 {
		t.Errorf("the two triggers posed %d play asks, want 2: the second trigger lost its target", asks)
	}
}
