package kgen

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// kdiscfiltBuild lowers and builds one card from its script text, returning
// its CardDef, the pool's filter table and refusals.
func kdiscfiltBuild(t *testing.T, name, src string) (kop.CardDef, *poolTables, []refusal) {
	t.Helper()
	card, diags := cards.ParseBytes("kdiscfilt_probe.txt", []byte(src))
	if card == nil {
		t.Fatalf("parse: %v", diags)
	}
	if d := card.Link(); len(d) != 0 {
		t.Fatalf("link: %v", d)
	}
	p := &pool{names: []string{name}, byName: map[string]*cards.Card{name: card}, nDecks: map[string]int{}, reg: &cards.Registry{}}
	lw := lowerPool(p)
	_, bs, pt, err := buildPool(lw)
	if err != nil {
		t.Fatal(err)
	}
	return bs[0].def, pt, lw.cards[0].Refusals
}

// kdiscfiltAb returns the card's ability whose cost carries flag.
func kdiscfiltAb(t *testing.T, d kop.CardDef, flag kop.CostFlags) *kop.Ability {
	t.Helper()
	for i := range d.Abilities {
		a := &d.Abilities[i]
		if a.Cost.Flags&flag != 0 {
			return a
		}
	}
	t.Fatalf("no ability with cost flag %v", flag)
	return nil
}

// TestDiscardFilteredCost: a filtered discard cost
// lowers to CostDiscard|CostDiscardFiltered with the candidates' hand filter;
// a named<self> clause is PredNamed over the card's own id; the filter's
// alternative split keeps a comma inside a named<Name> argument; a filtered
// discard beside another counted part refuses.
func TestDiscardFilteredCost(t *testing.T) {
	cell, pt, rs := kdiscfiltBuild(t, "Cell Probe", strings.Join([]string{
		"Name:Cell Probe",
		"ManaCost:R W",
		"Types:Artifact",
		"A:AB$ Draw | Cost$ 1 T Discard<1/Card.Legendary/legendary> | SpellDescription$ Cell Probe draws.",
	}, "\n"))
	if len(rs) != 0 {
		t.Fatalf("Cell Probe refused: %v", rs)
	}
	a := kdiscfiltAb(t, cell, kop.CostDiscardFiltered)
	if a.Cost.Flags&(kop.CostDiscard|kop.CostDiscardFiltered) != kop.CostDiscard|kop.CostDiscardFiltered {
		t.Errorf("Cell Probe cost flags %v", a.Cost.Flags)
	}
	if a.Cost.N != 1 {
		t.Errorf("Cell Probe cost N %d", a.Cost.N)
	}
	f := pt.filters[a.Cost.Filter]
	if f.Zone != kop.ZoneHand || f.Types&kop.TLegendary == 0 {
		t.Errorf("Cell Probe discard filter zone %v types %v", f.Zone, f.Types)
	}

	page, pt, rs := kdiscfiltBuild(t, "Page Probe", strings.Join([]string{
		"Name:Page Probe",
		"ManaCost:2",
		"Types:Legendary Artifact Creature Construct",
		"A:AB$ Draw | Cost$ Discard<1/Card.namedPage Probe> | SpellDescription$ Draw a card.",
	}, "\n"))
	if len(rs) != 0 {
		t.Fatalf("Page Probe refused: %v", rs)
	}
	a = kdiscfiltAb(t, page, kop.CostDiscardFiltered)
	if a.Cost.N != 1 || a.Cost.Flags&(kop.CostDiscard|kop.CostDiscardFiltered) != kop.CostDiscard|kop.CostDiscardFiltered {
		t.Errorf("Page Probe cost N %d flags %v", a.Cost.N, a.Cost.Flags)
	}
	f = pt.filters[a.Cost.Filter]
	if f.Pred != kop.PredNamed || f.PredN != 0 {
		t.Errorf("Page Probe filter Pred %v PredN %d, want PredNamed over its own id", f.Pred, f.PredN)
	}
	if f.Flags2&kop.F2SameNameAsSource != 0 {
		t.Errorf("Page Probe filter keeps F2SameNameAsSource")
	}

	noSub := func(string) bool { return false }
	alts := splitAlternatives("Card.namedFoo,Creature", noSub)
	if len(alts) != 2 || alts[0] != "Card.namedFoo" || alts[1] != "Creature" {
		t.Errorf("namedFoo,Creature split %q", alts)
	}
	alts = splitAlternatives("Card.namedPage, Loose Leaf", noSub)
	if len(alts) != 1 {
		t.Errorf("namedPage, Loose Leaf split %q, want one alternative", alts)
	}
	flt, err := parseFilter("Card.namedPage, Loose Leaf", noSub, noSub)
	if err != nil || len(flt.Clauses) != 1 || flt.Clauses[0].Flags&fNamedAsSource == 0 {
		t.Errorf("namedPage, Loose Leaf parse: %v %v", flt, err)
	}

	_, _, rs = kdiscfiltBuild(t, "Sac Probe", strings.Join([]string{
		"Name:Sac Probe",
		"ManaCost:1 B",
		"Types:Creature Zombie",
		"PT:2/2",
		"A:AB$ Draw | Cost$ Sac<1/Creature> Discard<1/Card.Legendary> | SpellDescription$ Draw a card.",
	}, "\n"))
	var got []string
	for _, r := range rs {
		got = append(got, r.Reason)
	}
	if all := strings.Join(got, " | "); !strings.Contains(all, "beside another counted part") {
		t.Errorf("sacrifice beside a filtered discard: refusals %q", all)
	}

	set := cardsettest.Set(t, "discfilt")
	// the generated matchup remaps the grandeur's PredN (a pool index) to
	// the CardID the named card has in the matchup: this matchup omits
	// every pool card before Grizzly Bears' place (Page, Loose Leaf is pool
	// index 6 and sorts between Grizzly Bears at 1 and Solitary Cell at 10),
	// so the identity remap the r2 finding hid behind reads the wrong card
	// (findings r2)
	tbl, idA, _, err := set.Matchup(
		[]cardset.Entry{{Name: "Page, Loose Leaf", Count: 1}, {Name: "Grizzly Bears", Count: 1}},
		[]cardset.Entry{{Name: "Solitary Cell", Count: 1}})
	if err != nil {
		t.Fatal(err)
	}
	if idA[0] == 6 {
		t.Fatalf("Page, Loose Leaf's matchup CardID is its pool index 6: the matchup no longer shifts the remap")
	}
	named := 0
	for i := range tbl.Filters {
		f := &tbl.Filters[i]
		if f.Pred == kop.PredNamed && f.Zone == kop.ZoneHand {
			named++
			if f.PredN != int16(idA[0]) {
				t.Errorf("grandeur filter PredN %d, want Page, Loose Leaf's matchup CardID %d", f.PredN, idA[0])
			}
		}
	}
	if named != 1 {
		t.Errorf("%d PredNamed hand filters, want exactly the grandeur's", named)
	}
	// a matchup that does not hold the named card: no object here can have
	// the name, so the filter's PredN is -1 and matches nothing (gorge
	// sharesName)
	tbl, _, _, err = set.Matchup(
		[]cardset.Entry{{Name: "Grizzly Bears", Count: 1}},
		[]cardset.Entry{{Name: "Solitary Cell", Count: 1}})
	if err != nil {
		t.Fatalf("matchup without the named card: err %v", err)
	}
	for i := range tbl.Filters {
		if f := &tbl.Filters[i]; f.Pred == kop.PredNamed && f.PredN != -1 {
			t.Errorf("matchup without the named card: filter %d PredN %d, want -1", i, f.PredN)
		}
	}
}
