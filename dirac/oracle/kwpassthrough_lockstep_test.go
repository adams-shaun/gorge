package oracle

import (
	"slices"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/cardset/cardsettest"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// kwPassThroughFocus is the T05 focus list: the heads the keyword
// pass-through newly consumes (Soulshift, Reinforce, Ingest) plus Annihilator,
// whose printed form already lowered, and the enablers their abilities need
// (Soulshift targets a Spirit card in the graveyard; the Eldrazi attack).
var kwPassThroughFocus = []FocusCard{
	{Name: "Body of Jukai", With: []string{"Burr Grafter", "Elder Pine of Jukai"}},
	{Name: "Deathknell Kami", With: []string{"Crawling Filth"}},
	{Name: "Moonlit Strider", With: []string{"Kami of the Honored Dead"}},
	{Name: "Break Ties"},
	{Name: "Swell of Courage"},
	{Name: "Brighthearth Banneret"},
	{Name: "Benthic Infiltrator", With: []string{"Warden of Geometries"}},
	{Name: "Ulamog's Crusher", With: []string{"Artisan of Kozilek"}},
}

// kwPassThroughPool deals the focus pool of the keyword pass-through's cards:
// tail-00's cards plus the focus list, built here like kfuzz -synth-with
// builds a pool.
func kwPassThroughPool(t *testing.T) (*Corpus, []PoolDeck, map[string]deck.File, cardset.Set) {
	t.Helper()
	reg := synthReg(t)
	dir, err := cardset.CorpusDir()
	if err != nil {
		t.Skipf("no corpus: %v", err)
	}
	main := cardsettest.Synth(t, "tail-00")
	names := slices.Clone(main.Names()[:main.NumCards()])
	for _, fc := range kwPassThroughFocus {
		names = append(names, fc.Name)
		names = append(names, fc.With...)
	}
	slices.Sort(names)
	names = slices.Compact(names)
	d, err := kgen.SynthUnionTable(names, dir, "kwpassthrough")
	if err != nil {
		t.Fatalf("union table: %v", err)
	}
	set := cardset.Raw(cardset.NewDataSet(d))
	pool, files, err := FocusPool(reg, set, "kwpassthrough", kwPassThroughFocus, FocusPerDeck, 20261119, 16)
	if err != nil {
		t.Fatalf("focus pool: %v", err)
	}
	c := &Corpus{Reg: reg, decks: map[string][]*cards.Card{}}
	c.AddDecks(files)
	return c, pool, files, set
}

// kwReachedHead counts the focus heads one game reached: the names the game's
// events name (cast, entered, died, returned, ...). A focus head never seen is
// a game that did not exercise the pass-through's lowering at all.
func kwReachedHead(t *testing.T, g *GorgeSide, heads map[string]bool) int {
	t.Helper()
	seen := map[string]bool{}
	for i := range g.E.L.Events {
		e := &g.E.L.Events[i]
		if o := g.E.G.Obj(e.Obj); o != nil && heads[cardName(o.Card)] {
			seen[cardName(o.Card)] = true
		}
	}
	return len(seen)
}

// TestKwPassThroughLockstep plays the focus pool gorge-vs-kernel: four pairs
// on both surfaces, then one four-seat pod. A divergence implicates a card the
// pass-through lowered (every other card of the union lowered before it). The
// games must also reach the focus cards: at least half the heads appear in a
// game's events.
func TestKwPassThroughLockstep(t *testing.T) {
	t.Parallel()
	c, pool, files, set := kwPassThroughPool(t)
	// precondition: the deal dealt every focus head at least twice (its own
	// head slot; filler may add more — a focus card is also a playable card
	// of the union)
	heads := map[string]bool{}
	for _, fc := range kwPassThroughFocus {
		heads[fc.Name] = true
	}
	copies := map[string]int{}
	for _, pd := range pool {
		f := files[pd.Name]
		for _, e := range f.Cards {
			if heads[e.Name] {
				copies[e.Name] += e.Count
			}
		}
	}
	for n := range heads {
		if copies[n] < 2 {
			t.Fatalf("%s dealt %d copies over the pool, want at least 2", n, copies[n])
		}
	}
	rc := RunConfig{MaxDecisions: 20000, MaxTurns: 100}
	reached := map[string]bool{}
	for i := 0; i < 4; i++ {
		for _, autopay := range []bool{false, true} {
			p := PairingAt(pool, i, 20261119)
			cfg := Config{GorgeAutopaySurface: autopay}
			m, tbl, err := SetMatchup(set, c, p.A, p.B, 0)
			if err != nil {
				t.Fatalf("pair %d: %v", i, err)
			}
			k, err := SetSide(set, tbl, cfg)(m, p.Seed)
			if err != nil {
				t.Fatalf("pair %d: kernel side: %v", i, err)
			}
			rcc := rc
			rcc.Config = cfg
			rcc.Finish = func(g *GorgeSide) {
				for n := range heads {
					if g.E == nil {
						continue
					}
					if kwReachedHead(t, g, map[string]bool{n: true}) == 1 {
						reached[n] = true
					}
				}
			}
			res := Lockstep(m, p.Seed, k, RandomPolicy(p.Seed), rcc)
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("pair %d (%s vs %s, surface autopay=%v): divergence at %d: %s %s %v",
					i, p.A, p.B, autopay, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
			}
			if !res.Over && res.Stall == "" {
				t.Fatalf("pair %d: game neither over nor stalled", i)
			}
			t.Logf("pair %d surface %v: %d decisions, turn %d, winner %d", i, autopay, res.Decisions, res.Turns, res.Winner)
		}
	}
	// one four-seat pod of the same pool
	pods := PodsFrom(pool, 0, 1, 20261119, 4)
	if len(pods) != 1 {
		t.Fatalf("%d pods, want 1", len(pods))
	}
	m, tbl, err := SetMatchupN(set, c, pods[0].Decks(), 0)
	if err != nil {
		t.Fatalf("pod: %v", err)
	}
	if m.NSeat() != 4 {
		t.Fatalf("pod: %d seats", m.NSeat())
	}
	k, err := SetSide(set, tbl, Config{})(m, pods[0].Seed)
	if err != nil {
		t.Fatalf("pod: kernel side: %v", err)
	}
	rcc := rc
	rcc.Finish = func(g *GorgeSide) {
		n := kwReachedHead(t, g, heads)
		t.Logf("pod reached %d focus heads", n)
	}
	res := Lockstep(m, pods[0].Seed, k, RandomPolicy(pods[0].Seed), rcc)
	CloseSide(k)
	if res.Div != nil {
		t.Fatalf("pod %v: divergence at %d: %s %s %v", pods[0].Decks(), res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff)
	}
	if !res.Over && res.Stall == "" {
		t.Fatalf("pod: game neither over nor stalled")
	}
	t.Logf("pod: %d decisions, winner %d", res.Decisions, res.Winner)
	if len(reached) < 4 {
		t.Fatalf("the games reached %d of 8 focus heads (%v): the pool does not exercise the pass-through", len(reached), reached)
	}
	t.Logf("reached %d of 8 focus heads: %v", len(reached), reached)
}
