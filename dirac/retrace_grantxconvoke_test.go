package dirac

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// TestRetraceGrantXConvokeNotBarred: an announced X and convoke apply to the
// spell whatever zone it is cast from, so Six's granted retrace on an {X} or a
// Convoke card is the plain one (printed base plus the discard), not a card
// the kernel withholds; Improvise is still withheld.
func TestRetraceGrantXConvokeNotBarred(t *testing.T) {
	deck := []cardset.Entry{{Name: "Six", Count: 1}, {Name: "Hill Giant", Count: 1}, {Name: "Forest", Count: 10}}
	base, da, _, err := cardsettest.Set(t, "offzonegrants").Matchup(deck, deck)
	if err != nil {
		t.Skip(err)
	}
	cases := []struct {
		name   string
		mut    func(cd *kop.CardDef)
		barred bool
	}{
		{"none", func(*kop.CardDef) {}, false},
		{"x", func(cd *kop.CardDef) { cd.Cost.X = 1 }, false},
		{"convoke", func(cd *kop.CardDef) { cd.Convoke = true }, false},
		{"improvise", func(cd *kop.CardDef) { cd.Improvise = true }, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tbl := *base
			tbl.Cards = slices.Clone(base.Cards)
			id := map[string]CardID{}
			for _, c := range da {
				id[tbl.Cards[c].Name] = c
			}
			tc.mut(&tbl.Cards[id["Hill Giant"]])
			if got := retraceGrantBarred(&prep{t: &tbl}, id["Hill Giant"]); got != tc.barred {
				t.Fatalf("retraceGrantBarred = %v, want %v", got, tc.barred)
			}
		})
	}
}
