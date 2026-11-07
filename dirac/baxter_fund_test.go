package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// Baxter Building's {4},{T} combination filter funded by a Forest pool and a
// two-colour land offers a 1RRG spell (CR 605.3b, 601.2g).
func TestBaxterFundedSpellOffered(t *testing.T) {
	set := cardsettest.Synth(t, "std-msh")
	deck := []cardset.Entry{{Name: "Alien Invasion", Count: 1}, {Name: "Baxter Building", Count: 1},
		{Name: "Fisk Tower", Count: 1}, {Name: "Mountain", Count: 1}, {Name: "Forest", Count: 1}}
	tbl, da, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for _, c := range da {
		id[tbl.Cards[c].Name] = c
	}
	sc := newScenTableLib(t, tbl, da[0], Config{NoElide: true})
	// the pair-380 board, in battlefield order
	sc.perm(id["Forest"], 0)
	fisk := sc.perm(id["Fisk Tower"], 0)
	sc.s.perms[sc.slot(fisk)].flags &^= pfTapped // it entered tapped; the board has since untapped it
	sc.perm(id["Forest"], 0)
	sc.perm(id["Baxter Building"], 0)
	sc.perm(id["Mountain"], 0)
	sc.perm(id["Forest"], 0)
	sc.hand(id["Alien Invasion"], 0)
	d := sc.s.Step()
	n := 0
	for i := 0; i < int(d.N); i++ {
		if d.Options[i].Kind() == AkCast {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("Alien Invasion cast offers %d, want 1 (opts %v)", n, d.Options[:d.N])
	}
}
