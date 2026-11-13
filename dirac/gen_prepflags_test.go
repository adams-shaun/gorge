package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
)

// genPrep.anyLure / anyMustBlock are false on a table with no lure,
// must-be-blocked or blocks-if-able card: prepLure/prepMustBlock read g.ix,
// which prepArena must have filled first.
func TestGenPrepBlockFlags(t *testing.T) {
	set := cardsettest.Set(t, "prepare")
	deck := []cardset.Entry{{Name: "Plains", Count: 8}, {Name: "Bake into a Pie", Count: 1}}
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	p := prepare(tbl)
	if p.gen == nil {
		t.Fatal("precondition: generated table has no genPrep")
	}
	for _, m := range []mech{mechLure, mechMustBlocked, mechBlocksIfAble} {
		if p.gen.ix[m] != arenaNone {
			t.Fatalf("precondition: mechanic %d is used by the table (ix=%d)", m, p.gen.ix[m])
		}
	}
	if p.gen.anyLure || p.gen.anyMustBlock {
		t.Fatalf("anyLure=%v anyMustBlock=%v with no lure/must-block/blocks-if-able card in the table", p.gen.anyLure, p.gen.anyMustBlock)
	}
}
