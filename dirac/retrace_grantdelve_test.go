package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// TestRetraceGrantDelveNotBarred: delve is no additional cost and no other
// cast route (CR 702.66a: it applies to the spell on the stack, whatever zone
// it was cast from), so Six's granted retrace on a Delve card is offered with
// the graveyard credit (offerRetraceCasts' delveOfferCredit, as gorge's
// offerCastable prices it) rather than withheld as a retracegrant stall.
func TestRetraceGrantDelveNotBarred(t *testing.T) {
	deck := []cardset.Entry{{Name: "Six", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Forest", Count: 10}}
	base, da, _, err := cardsettest.Set(t, "offzonegrants").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	tbl := *base // the card is mutated on a private copy of the table
	tbl.Cards = slices.Clone(base.Cards)
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	giant := id["Hill Giant"]
	tbl.Cards[giant].Delve = true
	sc := newScenTableLib(t, &tbl, id["Forest"], Config{NoElide: true})
	sc.perm(id["Six"], 0)
	sc.s.refreshTy(sc.k)
	grave := sc.grave(giant, 0, 1)[0]
	if !sc.s.retraceLive(sc.k, grave) {
		t.Fatalf("precondition: Six must grant the graveyard Hill Giant retrace")
	}
	if retraceGrantBarred(sc.k, giant) {
		t.Fatal("retraceGrantBarred: a Delve card's granted retrace is withheld")
	}
	var d Decision
	var av avail
	sc.s.offerRetraceCasts(sc.k, &d, 0, true, &av)
	if r := sc.s.StallReason(); r == StallRetraceGrant {
		t.Fatalf("stall %q on a Delve card's granted retrace", StallName(r))
	}
}
