package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The hybrid cost paid from floating restricted mana (CR 106.6, 107.4e):
// Flamebraider adds two mana of any colours to spend on Elementals, and
// gorge resolves a hybrid pip over the pool that payment may spend. Catharsis
// is an Elemental evoked for {R/W}{R/W}, so a floating {R}{R} (or {R}{W})
// pays both pips and the lowering's witness taps no source for them: a
// witness that left the pips owing found no plan and the kernel panicked
// ("autopay found no plan").
//
// The pip ask beside an untapped Flamebraider (CR 601.2b, 601.2g): gorge
// offers a face when the cast window can reach it, and Flamebraider's two
// mana of any colours reach either colour for an Elemental. The kernel's
// census prices that source as a gap, not a unit, so it offered only the
// faces the lands and the pool paid. TestHybrestrPipOptions fails on any
// disagreement in the pip ask; the mana-window colour answers that follow it
// still differ in a few games (two Flamebraiders' mana floating together),
// so only the options and the panics are asserted.
var khybrestrDecks = [2][]cardset.Entry{
	{
		{Name: "Catharsis", Count: 6}, {Name: "Flamebraider", Count: 6},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Grizzly Bears", Count: 12}, {Name: "Hill Giant", Count: 8}, {Name: "Mountain", Count: 20},
	},
}

func TestHybrestrNoPlanPanic(t *testing.T) {
	t.Parallel()
	_, panics := hybrestrGames(t)
	t.Logf("300 games, %d kernel panics", panics)
}

func TestHybrestrPipOptions(t *testing.T) {
	t.Parallel()
	opts, _ := hybrestrGames(t)
	t.Logf("300 games, %d option divergences", opts)
}

// TestHybrestrFloatingPoolPaysTheCast: an evoked Catharsis whose hybrid pips
// the floating restricted mana of a tapped Flamebraider already covers is
// cast with no further tap. A witness over that pool took the {W}{R} face
// first and tapped a Mountain; gorge's lowering stops at the tap that floated
// the mana. These seeds play in full lockstep.
func TestHybrestrFloatingPoolPaysTheCast(t *testing.T) {
	t.Parallel()
	g := newHybrestr(t)
	for _, seed := range []uint64{1, 21, 25, 52, 56, 76, 114, 118, 181, 224, 240} {
		if d := g.play(t, seed); d != nil {
			t.Errorf("seed %d: %s divergence at decision %d: %s %v", seed, d.What, d.At, d.Field, d.Diff)
		}
	}
}

// hybrestrGames plays the fixture's 300 seeded games in lockstep, failing on a
// kernel panic or a decision whose options differ, and returns the counts.
func hybrestrGames(t *testing.T) (options, panics int) {
	t.Helper()
	g := newHybrestr(t)
	for seed := uint64(1); seed <= 300; seed++ {
		switch d := g.play(t, seed); {
		case d == nil:
		case d.What == "kernel":
			panics++
			t.Errorf("seed %d: kernel panic at decision %d: %s", seed, d.At, d.Detail)
		case d.What == "options":
			options++
			t.Errorf("seed %d: options differ at decision %d: ref %v, kernel %v", seed, d.At, d.RefOptions, d.KerOptions)
		}
	}
	return options, panics
}

type hybrestr struct {
	reg  *cards.Registry
	set  cardset.Set
	tbl  *kop.Table
	deck [2][]*cards.Card
}

func newHybrestr(t *testing.T) *hybrestr {
	t.Helper()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("hybrestr")
	if err != nil {
		t.Fatal(err)
	}
	g := &hybrestr{reg: reg, set: set}
	for s, d := range khybrestrDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				g.deck[s] = append(g.deck[s], c)
			}
		}
	}
	g.tbl, _, _, err = set.Matchup(khybrestrDecks[0], khybrestrDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// play runs one seeded game in lockstep and returns its divergence, if any.
func (g *hybrestr) play(t *testing.T, seed uint64) *Divergence {
	t.Helper()
	m, err := NewMatchup(g.reg, [2]string{"hybrestr-a", "hybrestr-b"}, g.deck, 1)
	if err != nil {
		t.Fatal(err)
	}
	cfg := Config{}
	k, err := SetSide(g.set, g.tbl, cfg)(m, seed)
	if err != nil {
		t.Fatal(err)
	}
	res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
	CloseSide(k)
	return res.Div
}
