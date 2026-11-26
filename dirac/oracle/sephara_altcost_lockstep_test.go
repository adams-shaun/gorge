package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Sephara, Sky's Blade lockstep: {W} and four untapped flyers tapped
// rather than the {4}{W}{W}{W} mana cost (gorge AltCostIndex 1: an
// AlternativeCost static whose Cost$ is mana beside a tapXType part,
// CR 118.9, 601.2h). The deck is built to reach the offer: cheap flyers in
// quantity and an Iridescent Tiger to float the {W} (its enters trigger adds WUBRG): the oracle plays gorge's
// payment-planner surface, which skips AltCostIndex options (payment_plan.go),
// so the alternative cast is offered only while the floating pool pays its
// mana (cast_altfree.go offerAltFreeHand). Sephara sits in hand with fewer
// than four flyers on the table on some turns (the offer is withheld with
// three). The expected answer at every decision is gorge's.
var ksepharaaltcostDecks = [2][]cardset.Entry{
	{
		{Name: "Sephara, Sky's Blade", Count: 6}, {Name: "Suntail Hawk", Count: 12},
		{Name: "Iridescent Tiger", Count: 6}, {Name: "Plains", Count: 10}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8}, {Name: "Forest", Count: 20},
	},
}

func TestSepharaAltCostLockstep(t *testing.T) {
	t.Parallel()
	lockstepSet(t, "sepharaaltcost", ksepharaaltcostDecks, 600, "KSEPHARAALTCOST_SEEDS")
}

// TestSepharaAltCostLockstepMutation: with the alternative cost dropped some
// seed diverges (the pool casts Sephara for {W} over four tapped flyers).
func TestSepharaAltCostLockstepMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "sepharaaltcost", ksepharaaltcostDecks, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatAlternativeCost {
					abs[j].Static.Kind = kop.StatNone
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the alternative cost dropped")
	}
}

// TestSepharaAltCostLockstepThreeFlyersMutation: with the tapped set cut to
// three the kernel offers the alternative cast over three untapped flyers,
// which gorge withholds (it needs four), so some seed diverges: the clean
// lockstep above is what declines the cast with three.
func TestSepharaAltCostLockstepThreeFlyersMutation(t *testing.T) {
	t.Parallel()
	n := lockstepRun(t, "sepharaaltcost", ksepharaaltcostDecks, 300, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatAlternativeCost && abs[j].Cost.N == 4 {
					abs[j].Cost.N = 3
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	})
	if n == 0 {
		t.Fatal("no seed diverged with the alternative cost tapping three flyers")
	}
}
