package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Callaphe, Beloved of the Sea grants every creature and enchantment its
// controller controls "Spells your opponents cast that target this permanent
// cost {1} more to cast" (S:Mode$ Continuous | AddStaticAbility$ RaiseCost):
// a RaiseCost static whose Card.Self is each recipient, and whose Activator$
// Opponent is read against the recipient's controller (CR 601.2f, 613.7).
// kgen builds it into CardDef.Granted; the kernel's raiseCostGen and the
// target-dependent raise walks read it once per recipient (eachStaticG).
//
// The decks are U/R: Callaphe and Hill Giant are the recipients, Lightning
// Bolt the opponent's targeting spell, priced {R} plus {1} on a recipient and
// {R} on a player or an unrecipient, so the bolt's target menu and its
// affordability depend on the charge whenever the caster is tight on mana.
func grantedRaiseCostDecks() [2][]cardset.Entry {
	d := []cardset.Entry{
		{Name: "Callaphe, Beloved of the Sea", Count: 8},
		{Name: "Lightning Bolt", Count: 10},
		{Name: "Hill Giant", Count: 4},
		{Name: "Island", Count: 9},
		{Name: "Mountain", Count: 9},
	}
	return [2][]cardset.Entry{append([]cardset.Entry(nil), d...), append([]cardset.Entry(nil), d...)}
}

// grantedRaiseCostRun plays the seeds in lockstep on a table lowered from the
// deck's card names alone (printedDelveSet), so a spelling kgen still refuses
// fails the case at Matchup. It returns the divergent games; without report it
// stops at the first.
func grantedRaiseCostRun(t *testing.T, seeds uint64, report bool, mutate func(*kop.Table)) int {
	t.Helper()
	decks := grantedRaiseCostDecks()
	var names [2][]string
	for s, d := range decks {
		for _, e := range d {
			names[s] = append(names[s], e.Name)
		}
	}
	// synthReg first: it skips without a corpus, and printedDelveSet would
	// otherwise fatal on the missing .cards (a fresh clone must skip, not fail).
	reg := synthReg(t)
	set := printedDelveSet(t, names)
	var gd [2][]*cards.Card
	for s, d := range decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(gd[s]))
		}
	}
	_, casts, div := lockstepSeeds(t, reg, set, "granted_raisecost", decks, gd, 1, seeds, report, mutate, nil, castFirstPolicy)
	if casts == 0 {
		t.Fatal("no cast or activation in any game; the lockstep proves nothing")
	}
	return div
}

func TestGrantedRaiseCostLockstep(t *testing.T) {
	t.Parallel()
	grantedRaiseCostRun(t, 80, true, nil)
}

// TestGrantedRaiseCostLockstepMutation: with every card's Granted statics
// dropped the kernel no longer taxes a spell that targets a recipient, and
// some seed diverges (gorge keeps charging it).
func TestGrantedRaiseCostLockstepMutation(t *testing.T) {
	t.Parallel()
	if n := grantedRaiseCostRun(t, 80, false, grantedRaiseDropGranted); n == 0 {
		t.Fatal("no seed diverged with the granted RaiseCost dropped")
	}
}

// grantedRaiseDropGranted empties CardDef.Granted on every card.
func grantedRaiseDropGranted(tbl *kop.Table) {
	cards := append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cards {
		cards[i].Granted = nil
	}
	tbl.Cards = cards
}
