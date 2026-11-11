package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Dig NoLooking$ lockstep (CR 701.20a, 701.40a, 401.4): Telling Time,
// Moment of Truth and Expressive Iteration split the top three cards over two
// or three chained Digs whose NoLooking$ only withholds the chooser's look
// Note and the option labels in gorge's effDig, so the kernel plays them as
// plain Digs; Write into Being digs two, puts the chosen card on top, then
// manifests that remembered card (Manifest Defined$ Remembered) and offers the
// other on top of or below the library. Both seats play the same deck over the
// generated digblind pool, so every card is cast by both engines.

var kdbDeck = []cardset.Entry{
	{Name: "Telling Time", Count: 4}, {Name: "Moment of Truth", Count: 4},
	{Name: "Expressive Iteration", Count: 4}, {Name: "Write into Being", Count: 4},
	{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2},
	{Name: "Island", Count: 8}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 6},
}

const kdbMin = 4

// kdbScan counts, in gorge's event log, each carrier's cast and the cards
// manifested (a battlefield entry marked ManifestEntryCounter).
func kdbScan(g *GorgeSide, n map[string]int) {
	if g == nil {
		return
	}
	for _, ev := range g.E.L.Events {
		switch ev.Kind {
		case events.PutOnStack:
			if ob := g.E.G.Obj(ev.Obj); ob != nil && ob.Face() != nil && ev.From == state.ZHand {
				switch nm := ob.Face().Name; nm {
				case "Telling Time", "Moment of Truth", "Expressive Iteration", "Write into Being":
					n[nm]++
				}
			}
		case events.MoveZone:
			if ev.To == state.ZBattlefield && ev.Counter == events.ManifestEntryCounter {
				n["manifested"]++
			}
		}
	}
}

// runDigblind plays the seeded games; mutate turns Write into Being's
// manifest of the remembered card into a manifest of the library's top card
// and stops at the first divergence.
func runDigblind(t *testing.T, seeds uint64, mutate bool) (games, divs int, branch map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("digblind")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = kdbDeck
		for _, e := range kdbDeck {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(ents[0], ents[1])
	if err != nil {
		t.Fatal(err)
	}
	if mutate {
		cleared := 0
		for ci := range tbl.Cards {
			for ai := range tbl.Cards[ci].Abilities {
				for pi := range tbl.Cards[ci].Abilities[ai].Prog {
					in := &tbl.Cards[ci].Abilities[ai].Prog[pi]
					if in.Op == kop.OpManifest && in.B.N&kop.CfManifestRemembered != 0 {
						in.B = kop.Const(0)
						cleared++
					}
				}
			}
		}
		if cleared == 0 {
			t.Fatal("mutation found no Manifest Defined$ Remembered in the pool")
		}
	}
	played, branch := map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kdb-a", "kdb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		var gs *GorgeSide
		inner := eagerPolicy(seed, played)
		pol := PolicyFunc(func(c *PickCtx) Canon {
			if c.G != nil {
				gs = c.G
			}
			return inner.Pick(c)
		})
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		kdbScan(gs, branch)
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

func TestDigblindLockstep(t *testing.T) {
	t.Parallel()

	games, divs, branch := runDigblind(t, 200, false)
	var keys []string
	for k := range branch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, branch[k]))
	}
	t.Logf("%d games, %d divergences; casts %s", games, divs, strings.Join(bs, " "))
	for _, want := range []string{"Telling Time", "Moment of Truth", "Expressive Iteration", "Write into Being", "manifested"} {
		if branch[want] < kdbMin {
			t.Errorf("%s %d, want at least %d", want, branch[want], kdbMin)
		}
	}
}

// TestDigblindMutation proves the pool manifests the remembered card:
// manifesting the library's top card instead diverges.
func TestDigblindMutation(t *testing.T) {
	t.Parallel()

	games, divs, _ := runDigblind(t, 200, true)
	if divs == 0 {
		t.Fatalf("mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
