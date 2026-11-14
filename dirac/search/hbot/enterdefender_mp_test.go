//go:build dirac_mp

package hbot

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestEnterDefenderPodAnswersGorgesFirst: past two seats Anim Pakal's
// attack trigger asks which player its tapped-and-attacking Gnomes attack
// (CR 508.4), every option a player's target code. The bot answers gorge's
// first offer, the trigger's captured defender (chooseEnteringDefender's
// fallback), and reads no object for a player's code.
func TestEnterDefenderPodAnswersGorgesFirst(t *testing.T) {
	set := cardsettest.Set(t, "mbc")
	pakal := []cardset.Entry{{Name: "Anim Pakal, Thousandth Moon", Count: 16}, {Name: "Mountain", Count: 12}, {Name: "Plains", Count: 12}}
	walls := []cardset.Entry{{Name: "Fog Bank", Count: 10}, {Name: "Shipwreck Dowser", Count: 6}, {Name: "Island", Count: 24}}
	lands := []cardset.Entry{{Name: "Island", Count: 40}}
	// seat 0's blockers send some of seat 1's attacks past it, so the
	// captured defender is not always the kernel's canonical first option
	tbl, ds, err := set.MatchupN([][]cardset.Entry{walls, pakal, lands, lands})
	if err != nil {
		t.Fatal(err)
	}
	var decks [dirac.MaxSeats][]dirac.CardID
	copy(decks[:], ds)
	bt := New(NewFacts(tbl))
	asks, notFirst := 0, 0
	for seed := uint64(1); seed <= 20 && asks < 8; seed++ {
		var s dirac.State
		s.InitN(tbl, decks, 4, seed, dirac.Config{})
		var d dirac.Decision
		for i := 0; i < 3000; i++ {
			s.StepInto(&d)
			if d.Kind == dirac.DkNone {
				break
			}
			pick := bt.Pick(&s, &d)
			if d.Kind == dirac.DkChoose && d.Sub == dirac.ChGenEnterDefender {
				asks++
				players := 0
				for j := 0; j < int(d.N); j++ {
					if c := d.Options[j].Source(); c >= dirac.PlayerTarget0 && c < dirac.PlayerTarget0+dirac.MaxSeats {
						players++
					}
				}
				if players < 2 {
					t.Fatalf("seed %d: the ask offers %d players, want every opponent", seed, players)
				}
				if pick != 0 {
					notFirst++ // the fallback is not the kernel's canonical first
				}
				if got := s.FeatEnterDefenderIndex(d.Options[pick].Source()); got != 0 {
					t.Fatalf("seed %d: picked %#x, gorge's offer %d; want its first (the fallback)", seed, d.Options[pick].Source(), got)
				}
			}
			s.Apply(d.Options[pick])
		}
	}
	t.Logf("%d defender asks, %d answered past the canonical first", asks, notFirst)
	if asks == 0 || notFirst == 0 {
		t.Fatal("no defender ask told gorge's order from the canonical one: the walk is vacuous")
	}
}
