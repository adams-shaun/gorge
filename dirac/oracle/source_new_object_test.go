package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
)

// A Vampire Gourmand that died and returned with its attack trigger on the stack is a
// new object (CR 400.7), so its trigger can sacrifice it as "another creature": both
// engines offer the payment, and after the script both play on from it.
func TestReturnedSourcePaysAnotherLockstep(t *testing.T) {
	c, _ := testCorpus(t)
	r, err := ReadRepro("testdata/gourmand_returned.repro.json")
	if err != nil {
		t.Fatal(err)
	}
	m, err := r.Matchup(c.Reg)
	if err != nil {
		t.Fatal(err)
	}
	set := fdnSet()
	entries := func(names []string) []cardset.Entry {
		var es []cardset.Entry
		for _, n := range names {
			es = append(es, cardset.Entry{Name: n, Count: 1})
		}
		return es
	}
	tbl, _, _, err := set.Matchup(entries(r.Cards[0]), entries(r.Cards[1]))
	if err != nil {
		t.Fatal(err)
	}
	res := r.Rerun(m, SetSide(set, tbl, r.RunConfig().Config), 200)
	if res.Div != nil {
		t.Fatalf("divergence at %d: %s %s\n gorge %v\n dirac %v", res.Div.At, res.Div.What, res.Div.Field, res.Div.RefOptions, res.Div.KerOptions)
	}
}
