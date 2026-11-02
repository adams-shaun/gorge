package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/engine/cards"
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

// A Syr Alin killed under Undying Malice with his attack trigger on the stack returns as
// a new object, so "other creatures you control get +1/+0" reaches him: a resolving
// filter reads a returned source as new, as a cost does. Seed 644 reaches it.
func TestReturnedSourceIsOtherInAResolvingFilterLockstep(t *testing.T) {
	t.Parallel()
	reg := synthReg(t)
	set := cardsettest.Synth(t, "std-fdn")
	entries := []cardset.Entry{{Name: "Syr Alin, the Lion's Claw", Count: 12}, {Name: "Undying Malice", Count: 10},
		{Name: "Bake into a Pie", Count: 8}, {Name: "Plains", Count: 5}, {Name: "Swamp", Count: 5}}
	var deck []*cards.Card
	for _, e := range entries {
		cd, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, cd)
		}
	}
	tbl, _, _, err := set.Matchup(entries, entries)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(644); seed <= 650; seed++ {
		m, err := NewMatchup(reg, [2]string{"alin-a", "alin-b"}, [2][]*cards.Card{deck, deck}, 0)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{GorgeAutopaySurface: true})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: Config{GorgeAutopaySurface: true}})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: divergence at %d: %s %s %s %v", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Detail, res.Div.Diff)
		}
	}
}
