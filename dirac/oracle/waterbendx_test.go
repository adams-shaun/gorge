package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

const kwaterbendxKatara = "Katara, Water Tribe's Hope"

// kwaterbendxRun plays Katara, Water Tribe's Hope's Waterbend {X} (XMin$ 1)
// mirror in lockstep and returns the divergences and how many of Katara's
// activations paid with at least one waterbend tap (DebugWaterbendPaid).
func kwaterbendxRun(t *testing.T, report bool) (div, picked int) {
	t.Helper()
	deck := wbDeck(
		cardset.Entry{Name: kwaterbendxKatara, Count: 8}, cardset.Entry{Name: "Grizzly Bears", Count: 12},
		cardset.Entry{Name: "Plains", Count: 10}, cardset.Entry{Name: "Island", Count: 10},
	)
	katara := dirac.CardID(0xFFFF)
	find := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == kwaterbendxKatara {
				katara = dirac.CardID(i)
			}
		}
	}
	prev := dirac.DebugWaterbendPaid
	dirac.DebugWaterbendPaid = func(card dirac.CardID, picks, _ int) {
		if card == katara && picks > 0 {
			picked++
		}
	}
	defer func() { dirac.DebugWaterbendPaid = prev }()
	div = lockstepRun(t, "waterbendx", [2][]cardset.Entry{deck, deck}, 24, report, find)
	if katara == dirac.CardID(0xFFFF) {
		t.Fatalf("%s is not in the waterbendx table", kwaterbendxKatara)
	}
	return div, picked
}

// TestWaterbendXKataraActivates: Katara's Waterbend {X} is activated in
// lockstep, at least once with a waterbend tap paying its announced X
// (CR 701.67a, 601.2b): the X form's picks are asked before X, each pays one
// generic of the folded X, and X is offered only at or above the tap count.
func TestWaterbendXKataraActivates(t *testing.T) {
	if isolate(t) {
		return
	}
	_, picked := kwaterbendxRun(t, true)
	if picked == 0 {
		t.Fatal("no Katara activation paid with a waterbend tap: the run never reached the X form's picks")
	}
	t.Logf("%d Katara activations paid with waterbend taps", picked)
}
