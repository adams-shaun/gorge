package lockstep

import (
	"os"
	"testing"

	"github.com/adams-shaun/gorge/bots/botpolicy"
	"github.com/adams-shaun/gorge/engine/cardc/gorgebe"
	"github.com/adams-shaun/gorge/engine/cardc/mtgish"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/rules"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

func newBot(seed uint64) rules.Answerer {
	b := botpolicy.NewGameBot(seed)
	return func(e *rules.Engine, d *decision.Decision) decision.Intent { return b.Answer(e.G, e, d) }
}

// copySwap maps each distinct card of the games to a re-parsed copy of its own
// script, so every pointer changes and no definition does.
func copySwap(t *testing.T, games []Game) Swap {
	t.Helper()
	m := map[*cards.Card]*cards.Card{}
	var copies []*cards.Card
	for _, g := range games {
		for _, d := range g.Cfg.Decks {
			for _, c := range d {
				if _, ok := m[c]; ok {
					continue
				}
				text, err := os.ReadFile(c.Path)
				if err != nil {
					t.Fatal(err)
				}
				cp, diags := gorgebe.ParseScript(c.Path, text)
				if cp == nil || len(diags) > 0 {
					t.Fatalf("%s: %v", c.Path, diags)
				}
				m[c] = cp
				copies = append(copies, cp)
			}
		}
	}
	if err := gorgebe.Bind(copies); err != nil {
		t.Fatal(err)
	}
	return func(d []*cards.Card) []*cards.Card {
		out := make([]*cards.Card, len(d))
		for i, c := range d {
			out[i] = c
			if cp, ok := m[c]; ok {
				out[i] = cp
			}
		}
		return out
	}
}

func TestSwapIsNeutral(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	games, err := FDN(root, forge, 0, 5)
	if err != nil {
		t.Fatal(err)
	}
	names, decks, err := testutil.AcceptanceDecks(forge, 2)
	if err != nil {
		t.Fatal(err)
	}
	games = append(games, Legacy(forge, names, decks))
	swap := copySwap(t, games)
	for _, g := range games {
		for _, d := range []Direction{Forward, Reverse} {
			r := Run(g, d, swap, newBot)
			t.Logf("%s %s swapped=%d %s seq=%d", r.ID, r.Dir, r.Swapped, r.Outcome, r.Seq)
			if r.Outcome != "clean" || r.Swapped == 0 {
				t.Errorf("%s %s: outcome %s swapped %d seq %d: %s", r.ID, r.Dir, r.Outcome, r.Swapped, r.Seq, r.Detail)
			}
		}
	}
}

func TestHarnessFindsDivergence(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	forest, ok := forge.Lookup("Forest")
	if !ok {
		t.Fatal("no Forest")
	}
	bear := func(pt string) *cards.Card {
		c, diags := gorgebe.ParseScript("zz/bear.txt", []byte("Name:Zz Bear\nManaCost:G\nTypes:Creature Bear\nPT:"+pt+"\n"))
		if c == nil || len(diags) > 0 {
			t.Fatalf("%v", diags)
		}
		if err := gorgebe.Bind([]*cards.Card{c}); err != nil {
			t.Fatal(err)
		}
		return c
	}
	two, three := bear("2/2"), bear("3/3")
	deck := func() []*cards.Card {
		var d []*cards.Card
		for i := 0; i < 40; i++ {
			d = append(d, forest)
		}
		for i := 0; i < 20; i++ {
			d = append(d, two)
		}
		return d
	}
	g := Game{ID: "zz/bears", Cfg: rules.Config{Seed: 1, Names: []string{"A", "B"}, Decks: [][]*cards.Card{deck(), deck()},
		Tokens: forge.Tokens, NameUniverse: forge.Universe(), Mulligans: 1, LoopGuard: rules.WithDefaultObjectCap(nil)}}
	mapper := func(to *cards.Card) Swap {
		return func(d []*cards.Card) []*cards.Card {
			out := make([]*cards.Card, len(d))
			for i, c := range d {
				out[i] = c
				if c == two {
					out[i] = to
				}
			}
			return out
		}
	}
	for _, dir := range []Direction{Forward, Reverse} {
		if r := Run(g, dir, mapper(three), newBot); r.Outcome != "diverged" || r.Seq == 0 {
			t.Errorf("%s 3/3: %+v", dir, r)
		}
		if r := Run(g, dir, mapper(two), newBot); r.Outcome != "clean" {
			t.Errorf("%s identity: %+v", dir, r)
		}
	}
}

func TestFDNSliceLoads(t *testing.T) {
	forge := testutil.CorpusRegistry(t)
	root, err := mtgish.ModuleRoot()
	if err != nil {
		t.Fatal(err)
	}
	all, err := FDN(root, forge, 0, 1500)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 1500 {
		t.Fatalf("got %d games", len(all))
	}
	for _, g := range all {
		if len(g.Cfg.Decks) != 2 || len(g.Cfg.Decks[0]) == 0 || len(g.Cfg.Decks[1]) == 0 || g.Cfg.Seed == 0 {
			t.Fatalf("%s: bad game", g.ID)
		}
	}
	tail, err := FDN(root, forge, 1490, 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(tail) != 10 || tail[0].ID != "fdn/1490" {
		t.Errorf("tail %d games", len(tail))
	}
}
