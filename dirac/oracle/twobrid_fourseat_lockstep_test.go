package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// twobridCorpusCards is the census of corpus cards printing a {2/X} pip.
// playable is the subset the generated twobrid pool plays; refused is the
// rest (the pool refuses the card by name), so the log states the whole
// census and which shapes the lockstep can reach.
func twobridCorpusCards(t *testing.T, set cardset.Set) (playable, refused []string) {
	t.Helper()
	reg := synthReg(t)
	seen := map[string]bool{}
	for _, c := range reg.AllCards() {
		for _, f := range c.Faces {
			if !tbCard(f) || f.Name == "" {
				continue
			}
			if seen[f.Name] {
				break
			}
			seen[f.Name] = true
			if _, ok := set.Lookup(f.Name); ok {
				playable = append(playable, f.Name)
			} else {
				refused = append(refused, f.Name)
			}
			break
		}
	}
	slices.Sort(playable)
	slices.Sort(refused)
	return playable, refused
}

// twobridFourSeatDeck is one 40-card deck: every twobrid corpus card once,
// then a spread of basics.
func twobridFourSeatDeck(t *testing.T, set cardset.Set) ([]string, []string, []string) {
	t.Helper()
	tw, refused := twobridCorpusCards(t, set)
	if len(tw) == 0 {
		t.Fatal("the census found no twobrid card the set plays")
	}
	deck := append([]string{}, tw...)
	basics := []string{"Plains", "Island", "Swamp", "Mountain", "Forest"}
	for len(deck) < 40 {
		deck = append(deck, basics[len(deck)%len(basics)])
	}
	return deck[:40], tw, refused
}

// twobridFourSeat plays seats of deck against each other at four seats under
// the twobrid policy, returning the first divergence and the seeds played.
func twobridFourSeat(t *testing.T, set cardset.Set, deck []string, seeds uint64, mutate func(*kop.Table)) (int, *Divergence) {
	t.Helper()
	reg := synthReg(t)
	var list []cardset.Entry
	var cardsDeck []*cards.Card
	for _, n := range deck {
		list = append(list, cardset.Entry{Name: n, Count: 1})
		c, ok := reg.Lookup(n)
		if !ok {
			t.Fatalf("%s not in the corpus", n)
		}
		cardsDeck = append(cardsDeck, c)
	}
	lists := [][]cardset.Entry{list, list, list, list}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	div := 0
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchupN(reg, []string{"a", "b", "c", "d"},
			[][]*cards.Card{cardsDeck, cardsDeck, cardsDeck, cardsDeck}, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%2 == 0 {
			cfg.GorgeAutopaySurface = true
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := ktwobridPolicy(seed, nil)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 25})
		CloseSide(k)
		if res.Div != nil {
			div++
			return div, res.Div
		}
		if res.StallUnmodelled || res.Stall != "" {
			return div, &Divergence{At: res.Decisions, What: "stall", Field: res.Stall}
		}
	}
	return div, nil
}

// TestTwobridFourSeatLockstep: every corpus twobrid card plays the same game
// at four seats, under both oracle surfaces, over the generated twobrid pool.
func TestTwobridFourSeatLockstep(t *testing.T) {
	t.Parallel()
	cardsettest.Require(t, "twobrid")
	set := cardsettest.Set(t, "twobrid")
	deck, tw, refused := twobridFourSeatDeck(t, set)
	if len(tw) < 10 {
		t.Fatalf("the census found %d twobrid cards the pool plays, want the pool's shapes", len(tw))
	}
	t.Logf("twobrid census: %d corpus cards, %d pool-playable %v, refused %v", len(tw)+len(refused), len(tw), tw, refused)
	const seeds = 24
	if div, d := twobridFourSeat(t, set, deck, seeds, nil); div != 0 {
		t.Fatalf("diverged: %+v", d)
	}
}

// TestTwobridFourSeatMutation: rewriting every twobrid pip must diverge, so
// the lockstep above is exercising the mechanism rather than passing on decks
// that never cast one.
func TestTwobridFourSeatMutation(t *testing.T) {
	t.Parallel()
	cardsettest.Require(t, "twobrid")
	set := cardsettest.Set(t, "twobrid")
	deck, _, _ := twobridFourSeatDeck(t, set)
	mutate := func(tbl *kop.Table) {
		for i := range tbl.Cards {
			c := &tbl.Cards[i]
			if c.Cost.Tb == 0 {
				continue
			}
			for j, n := 0, c.Cost.TbN(); j < n; j++ {
				c.Cost.Pips[c.Cost.TbCol(j)]++
			}
			c.Cost.Tb, c.Cost.Hyb[0] = 0, 0
		}
	}
	if div, d := twobridFourSeat(t, set, deck, 24, mutate); div == 0 {
		t.Fatalf("the every-pip-its-colour mutation never diverged: %v", d)
	}
}
