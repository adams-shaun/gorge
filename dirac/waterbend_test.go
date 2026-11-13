package dirac

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
)

// The Waterbend cost (CR 701.67) unit scenarios over the generated tlawater
// pool: the offer credit (raise_cost_extra.go withWaterbendOfferCredit), the
// N-pick cap (pay.ConvokeAsk), and the hold that keeps a picked mana creature
// out of the mana plan (castpayment.go Committed).

func wbTable(t *testing.T, names ...string) (*kop.Table, map[string]CardID) {
	t.Helper()
	var deck []cardset.Entry
	for _, n := range names {
		deck = append(deck, cardset.Entry{Name: n, Count: 1})
	}
	set := cardsettest.Set(t, "tlawater")
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	id := map[string]CardID{}
	for i := range tbl.Cards {
		id[tbl.Cards[i].Name] = CardID(i)
	}
	return tbl, id
}

func wbOffers(t *testing.T, sc *scen, src Obj) bool {
	t.Helper()
	d := sc.s.Step()
	for i := 0; i < int(d.N); i++ {
		if a := d.Options[i]; a.Kind() == AkActivate && a.Source() == TCode(uint8(src)) {
			return true
		}
	}
	return false
}

func TestWaterbendOfferCredit(t *testing.T) {
	t.Run("source is credited", func(t *testing.T) {
		tbl, id := wbTable(t, "Flexible Waterbender", "Grizzly Bears", "Plains")
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		fw := sc.perm(id["Flexible Waterbender"], 0)
		sc.perm(id["Grizzly Bears"], 0)
		sc.perm(id["Grizzly Bears"], 0)
		if !wbOffers(t, sc, fw) {
			t.Fatal("Waterbend<3> withheld with two creatures and its source, whose cost has no {T}")
		}
	})
	t.Run("taps and sources pay together", func(t *testing.T) {
		tbl, id := wbTable(t, "Flexible Waterbender", "Grizzly Bears", "Plains")
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		fw := sc.perm(id["Flexible Waterbender"], 0)
		sc.perm(id["Grizzly Bears"], 0)
		sc.perm(id["Plains"], 0)
		if !wbOffers(t, sc, fw) {
			t.Fatal("Waterbend<3> withheld with two credited creatures and a Plains")
		}
	})
	t.Run("mana creature is not credited", func(t *testing.T) {
		tbl, id := wbTable(t, "Flexible Waterbender", "Llanowar Elves", "Plains")
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		fw := sc.perm(id["Flexible Waterbender"], 0)
		sc.perm(id["Llanowar Elves"], 0)
		if wbOffers(t, sc, fw) {
			t.Fatal("Waterbend<3> offered with its source and a mana creature; the mana source pays one and earns no credit")
		}
	})
	t.Run("three credited creatures", func(t *testing.T) {
		tbl, id := wbTable(t, "Flexible Waterbender", "Grizzly Bears", "Plains")
		sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
		fw := sc.perm(id["Flexible Waterbender"], 0)
		for i := 0; i < 3; i++ {
			sc.perm(id["Grizzly Bears"], 0)
		}
		if !wbOffers(t, sc, fw) {
			t.Fatal("Waterbend<3> not offered with three credited creatures and no mana")
		}
	})
}

func TestWaterbendPicksCapN(t *testing.T) {
	tbl, id := wbTable(t, "Flexible Waterbender", "Grizzly Bears", "Plains")
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	fw := sc.perm(id["Flexible Waterbender"], 0)
	var bears []Obj
	for i := 0; i < 4; i++ {
		bears = append(bears, sc.perm(id["Grizzly Bears"], 0))
	}
	sc.do(t, AkActivate, TCode(fw), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChWaterbend {
		t.Fatalf("first decision %v/%d, want the waterbend ask", d.Kind, d.Sub)
	}
	for i := 0; i < 3; i++ {
		sc.do(t, AkChoose, TCode(bears[i]), scAny)
		d = sc.s.Step()
	}
	if d.Kind == DkChoose && d.Sub == ChWaterbend {
		t.Fatalf("a fourth pick was offered: Waterbend<3> must cap at three")
	}
	for i := 0; i < 3; i++ {
		if sc.s.perms[sc.slot(bears[i])].flags&pfTapped == 0 {
			t.Fatalf("pick %d was not tapped", i)
		}
	}
	if sc.s.perms[sc.slot(bears[3])].flags&pfTapped != 0 {
		t.Fatal("the unpicked creature was tapped")
	}
	if sc.s.perms[sc.slot(fw)].flags&pfTapped != 0 {
		t.Fatal("the source was tapped")
	}
}

func TestWaterbendNoAskWithoutCandidates(t *testing.T) {
	tbl, id := wbTable(t, "Aang's Iceberg", "Plains")
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	ib := sc.perm(id["Aang's Iceberg"], 0)
	sc.lands(0, id["Plains"], 3)
	sc.do(t, AkActivate, TCode(ib), scAny)
	d := sc.s.Step()
	if d.Kind == DkChoose && d.Sub == ChWaterbend {
		t.Fatalf("waterbend asked with no artifact or creature candidate: %v", d.Options[:d.N])
	}
}

func TestWaterbendPickedManaCreatureNotTappedForMana(t *testing.T) {
	tbl, id := wbTable(t, "Flexible Waterbender", "Grizzly Bears", "Llanowar Elves", "Forest", "Plains")
	sc := newScenTableLib(t, tbl, id["Plains"], Config{NoElide: true})
	fw := sc.perm(id["Flexible Waterbender"], 0)
	elf := sc.perm(id["Llanowar Elves"], 0)
	b1 := sc.perm(id["Grizzly Bears"], 0)
	b2 := sc.perm(id["Grizzly Bears"], 0)
	forest := sc.perm(id["Forest"], 0)
	sc.do(t, AkActivate, TCode(fw), scAny)
	d := sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChWaterbend {
		t.Fatalf("first decision %v/%d, want the waterbend ask", d.Kind, d.Sub)
	}
	sc.do(t, AkChoose, TCode(elf), scAny)
	sc.s.Step()
	sc.do(t, AkChoose, TCode(b1), scAny)
	d = sc.s.Step()
	if d.Kind != DkChoose || d.Sub != ChWaterbend {
		t.Fatalf("decision after two picks %v/%d, want the waterbend ask", d.Kind, d.Sub)
	}
	sc.do(t, AkDone, scAny, scAny)
	sc.s.Step()
	if sc.s.perms[sc.slot(forest)].flags&pfTapped == 0 {
		t.Fatal("the plan did not tap the Forest; it tapped the picked mana creature instead")
	}
	if sc.s.perms[sc.slot(elf)].flags&pfTapped == 0 {
		t.Fatal("the picked mana creature was not tapped at payment")
	}
	if sc.s.perms[sc.slot(b2)].flags&pfTapped != 0 {
		t.Fatal("the unpicked creature was tapped")
	}
	if sc.s.perms[sc.slot(fw)].flags&pfTapped != 0 {
		t.Fatal("the source was tapped")
	}
}
