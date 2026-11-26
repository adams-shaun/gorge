package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// grantSearchPainlandDecks: Cryptolith Rite's granted any-colour mana ability
// beside School of the Unseen's filter ({2},{T}: two mana of one colour),
// Karplusan Forest and Mountains funding Dauthi Embrace's {B}{B} play.
var grantSearchPainlandDecks = [2][]cardset.Entry{
	{
		{Name: "Cryptolith Rite", Count: 4}, {Name: "Dauthi Embrace", Count: 4}, {Name: "School of the Unseen", Count: 4},
		{Name: "Karplusan Forest", Count: 4}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 12}, {Name: "Grizzly Bears", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 16},
	},
}

// grantSearchPainlandSeeds are the games of the 1..300 castFirstPolicy sweep
// whose first divergence was the kernel offering a play gorge left unpriced
// (an "options options" divergence) before a painland funded the grant search
// with its painful colours.
var grantSearchPainlandSeeds = []uint64{10, 14, 21, 23, 41, 59, 145, 170, 218}

// TestGrantSearchPainlandFundingLockstep: a painland funding a granted-mana
// prefix search asks its painful colours too (gorge's mana ask for Karplusan
// Forest answers {C}, {R} and {G}, each a node of the potentialScriptNodes budget, CR
// 605.3a/605.3b), so the kernel's search reaches no funding line gorge's
// budget ran out before.
func TestGrantSearchPainlandFundingLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	var names [2][]string
	var gd [2][]*cards.Card
	for s, d := range grantSearchPainlandDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				names[s] = append(names[s], e.Name)
				gd[s] = append(gd[s], c)
			}
		}
	}
	set := cardset.Raw(printedDelveSet(t, names))
	for _, seed := range grantSearchPainlandSeeds {
		_, _, divs := lockstepSeedsMode(t, reg, set, "printed-delve", grantSearchPainlandDecks, gd, seed, seed, lockstepQuiet, nil, nil, castFirstPolicy)
		for _, d := range divs {
			t.Error(d.msg)
		}
	}
}
