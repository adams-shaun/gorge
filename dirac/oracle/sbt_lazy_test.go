package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/rules"
)

// TestSBTCardsResolvesLazily pins the resident-memory fix: building an
// SBTCards over a table materialises no card (the old constructor resolved
// every table card and built a whole-table scratch engine), and touching
// one card materialises at most that card.
func TestSBTCardsResolvesLazily(t *testing.T) {
	t.Parallel()

	testCorpus(t) // skips without a corpus
	dir := corpusDirOrSkip(t)
	// Its own registry: MaterializedCount on the shared one also counts what
	// every parallel test looks up.
	reg, err := cards.OpenCorpus(dir)
	if err != nil {
		t.Fatal(err)
	}
	set := cardsettest.Raw(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	before := reg.MaterializedCount()
	cs := NewSBTCards(reg, tbl, nil)
	if got := reg.MaterializedCount(); got != before {
		t.Fatalf("NewSBTCards materialized %d cards, want none", got-before)
	}
	cd := cs.card(0)
	if cd == nil || cd.name == "" {
		t.Fatalf("card 0: unresolved (%v)", cd)
	}
	if got := reg.MaterializedCount(); got > before+1 {
		t.Fatalf("first card materialized %d cards, want at most 1", got-before)
	}
	cs.card(0)
	if got := reg.MaterializedCount(); got > before+1 {
		t.Fatalf("second touch materialized more cards (%d)", got-before)
	}
}

// TestSBTCardsViewMatchesWholeTable pins the lazy per-card view against the
// old whole-table scratch engine: keywords and P/T off the battlefield must
// be identical for every resolved table card. The named fixtures (a card
// with printed keywords, and a CDA card) keep a silently empty table from
// passing.
func TestSBTCardsViewMatchesWholeTable(t *testing.T) {
	t.Parallel()

	c, _ := testCorpus(t)
	set := cardsettest.Raw(t, "fdn")
	tbl, err := set.Pool()
	if err != nil {
		t.Fatal(err)
	}
	cs := NewSBTCards(c.Reg, tbl, nil)
	var deck []*cards.Card
	at := make([]int, len(tbl.Cards))
	for i := range tbl.Cards {
		at[i] = -1
		if gc := cs.card(dirac.CardID(i)).gc; gc != nil {
			at[i] = len(deck)
			deck = append(deck, gc)
		}
	}
	e := rules.New(rules.Config{Seed: 1, Names: []string{"p0", "p1"}, Decks: [][]*cards.Card{deck, deck}, Tokens: c.Reg.Tokens})
	kw, cda := 0, 0
	for i := range tbl.Cards {
		sc := cs.card(dirac.CardID(i))
		if sc.gc == nil {
			continue
		}
		_, _, kws, p, tou := e.ViewCharacteristics(e.G.Objs[at[i]].ID)
		v := cs.view(dirac.CardID(i))
		if !slices.Equal(v.kw, kws) || v.pow != p || v.tou != tou {
			t.Fatalf("card %d %q: lazy kw=%v p/t=%d/%d, whole-table kw=%v p/t=%d/%d",
				i, sc.name, v.kw, v.pow, v.tou, kws, p, tou)
		}
		if len(v.kw) > 0 {
			kw++
		}
		for j := range tbl.Cards[i].Abilities {
			if a := &tbl.Cards[i].Abilities[j]; a.Kind == kop.AbStatic && a.Static.Kind == kop.StatCDAPT {
				cda++
				break
			}
		}
	}
	if kw < 20 || cda == 0 {
		t.Fatalf("fixture too weak: %d cards with keywords, %d CDA cards", kw, cda)
	}
	// Serra Angel: printed Flying and Vigilance, 4/4 off the battlefield.
	if v := cs.view(tableCard(t, tbl, "Serra Angel")); !slices.Contains(v.kw, "Flying") ||
		!slices.Contains(v.kw, "Vigilance") || v.pow != 4 || v.tou != 4 {
		t.Fatalf("Serra Angel: kw=%v p/t=%d/%d, want Flying+Vigilance 4/4", v.kw, v.pow, v.tou)
	}
	// Consuming Aberration: a */* CDA, 0/0 with empty graveyards.
	if v := cs.view(tableCard(t, tbl, "Consuming Aberration")); v.pow != 0 || v.tou != 0 {
		t.Fatalf("Consuming Aberration CDA: p/t=%d/%d, want 0/0", v.pow, v.tou)
	}
}

// tableCard is the CardID of the table's card named name.
func tableCard(t *testing.T, tbl *kop.Table, name string) dirac.CardID {
	t.Helper()
	for i := range tbl.Cards {
		if tbl.Cards[i].Name == name {
			return dirac.CardID(i)
		}
	}
	t.Fatalf("table has no card %q", name)
	return 0
}
