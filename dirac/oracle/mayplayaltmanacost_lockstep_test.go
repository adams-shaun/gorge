package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// A printed MayPlay$ static's MayPlayAltManaCost$ in lockstep (dirac/kgen
// lower_static.go mayPlayAltManaCost): Bolas's Citadel lets its controller
// play lands and cast spells from the top of the library, a spell paid for
// with life equal to its mana value rather than its mana cost (CR 118.9). The
// policy of libtop_test.go plays the top card whenever it is offered, so the
// offered card, its price and the life paid are checked against gorge's at
// every decision.
var mayPlayAltManaCostDecks = [2][]cardset.Entry{
	{
		{Name: "Bolas's Citadel", Count: 6}, {Name: "Walking Corpse", Count: 8}, {Name: "Doom Blade", Count: 4}, {Name: "Swamp", Count: 22},
	},
	{
		{Name: "Walking Corpse", Count: 8}, {Name: "Doom Blade", Count: 4}, {Name: "Swamp", Count: 28},
	},
}

func TestMayPlayAltManaCostLockstep(t *testing.T) {
	t.Parallel()
	var st klibtopStats
	n := lockstepRunPolicy(t, "mayplayaltmanacost", mayPlayAltManaCostDecks, 80, true, nil, nil,
		func(seed uint64) Policy { return klibtopPolicy(seed, &st) })
	if n != 0 {
		t.Fatalf("%d seeds diverged", n)
	}
	if st.libCasts == 0 || st.libLands == 0 {
		t.Fatalf("Citadel's permission was not exercised: %d spells, %d lands cast or played from the library", st.libCasts, st.libLands)
	}
	t.Logf("from the library: %d spells, %d lands", st.libCasts, st.libLands)
}

// TestMayPlayAltManaCostLockstepMutation: with the life alternative cleared
// from every static the permission is a plain mana cast, and some seed
// diverges from gorge's life-priced offer.
func TestMayPlayAltManaCostLockstepMutation(t *testing.T) {
	t.Parallel()
	var st klibtopStats
	n := lockstepRunPolicy(t, "mayplayaltmanacost", mayPlayAltManaCostDecks, 80, false, func(tbl *kop.Table) {
		cards := append([]kop.CardDef(nil), tbl.Cards...)
		for i := range cards {
			abs := append([]kop.Ability(nil), cards[i].Abilities...)
			for j := range abs {
				if abs[j].Kind == kop.AbStatic && abs[j].Static.Kind == kop.StatMayPlayFrom {
					abs[j].Static.StFlags &^= kop.StMayPayLife
				}
			}
			cards[i].Abilities = abs
		}
		tbl.Cards = cards
	}, nil, func(seed uint64) Policy { return klibtopPolicy(seed, &st) })
	if n == 0 {
		t.Fatal("no seed diverged with the life alternative cleared")
	}
}
