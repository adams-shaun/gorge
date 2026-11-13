package oracle

import (
	"fmt"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The each-player library search lockstep (ChangeZone DefinedPlayer$ Player,
// CR 701.23a): gorge effSearchLibrary walks every alive player from the
// controller, and each player confirms (Optional$), picks from and shuffles
// their own library. dirac lowers it as a one-instruction OpRepeatEachPlayer
// body (C&1: controller first) over an OpSearchEx of RefEach
// (dirac/kgen/lower_eachsearch.go). Field of Ruin and Veteran Explorer are the
// headline carriers, Jungle Wayfinder and Noble Benefactor ride along, and
// Shock kills the Explorer. Both seats play the same deck over the generated
// eachsearch pool.

var kesDeck = []cardset.Entry{
	{Name: "Field of Ruin", Count: 4}, {Name: "Veteran Explorer", Count: 4},
	{Name: "Jungle Wayfinder", Count: 3}, {Name: "Noble Benefactor", Count: 3},
	{Name: "Shock", Count: 4},
	{Name: "Forest", Count: 10}, {Name: "Mountain", Count: 6}, {Name: "Island", Count: 6},
}

// kesCarriers are the deck's each-player searches; kesHeadline the two that
// must reach kesMin asks per seat.
var (
	kesCarriers = []string{"Field of Ruin", "Jungle Wayfinder", "Noble Benefactor", "Veteran Explorer"}
	kesHeadline = []string{"Field of Ruin", "Veteran Explorer"}
)

const (
	kesMin   = 3  // search asks each headline carrier must pose to each seat
	kesSeeds = 60 // games of the lockstep run
)

// kesPolicy plays a land when it can, then casts eagerly (eagerPolicy), and
// tallies gorge's search and search_confirm asks by source card and asked
// seat ("<card>/p<seat>"). A multi-card pick's later rounds are one ask.
func kesPolicy(seed uint64, played map[string]int, n map[string]int) Policy {
	inner := eagerPolicy(seed, played)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, o := range c.D.Options {
			if o.Kind() == ActPlayLand {
				return o
			}
		}
		a := inner.Pick(c)
		if c.G == nil {
			return a
		}
		d := c.G.Pending()
		if d == nil || d.ResumeKind != "search" && d.ResumeKind != "search_confirm" || c.G.MidPick() {
			return a
		}
		src := c.G.E.G.Obj(d.Source)
		if src == nil || src.Face() == nil {
			return a
		}
		n[fmt.Sprintf("%s/p%d", src.Face().Name, d.Player)]++
		return a
	})
}

// runEachSearch plays the seeded games. mutate rewrites every OpSearchEx of
// RefEach to RefController (the controller searching once per player) and
// stops at the first divergence.
func runEachSearch(t *testing.T, seeds uint64, mutate bool) (games, divs int, n map[string]int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("eachsearch")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var ents [2][]cardset.Entry
	for s := range decks {
		ents[s] = kesDeck
		for _, e := range kesDeck {
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
	// Precondition: every carrier lowered to the each-player loop, an
	// OpSearchEx of RefEach right behind an OpRepeatEachPlayer of one
	// instruction.
	loops := 0
	for ci := range tbl.Cards {
		for ai := range tbl.Cards[ci].Abilities {
			p := tbl.Cards[ci].Abilities[ai].Prog
			for pi := range p {
				if p[pi].Op != kop.OpSearchEx || p[pi].Obj != kop.RefEach {
					continue
				}
				if pi == 0 || p[pi-1].Op != kop.OpRepeatEachPlayer || p[pi-1].C&1 == 0 {
					t.Fatalf("an OpSearchEx of RefEach outside a controller-first OpRepeatEachPlayer")
				}
				loops++
				if mutate {
					p[pi].Obj = kop.RefController
				}
			}
		}
	}
	if loops != len(kesCarriers) {
		t.Fatalf("%d each-player searches in the table, want one per carrier (%d)", loops, len(kesCarriers))
	}
	n = map[string]int{}
	played := map[string]int{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kes-a", "kes-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kesPolicy(seed, played, n), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		games++
		if err := unmodelledStallError(t.Name(), &res); err != nil {
			t.Errorf("seed %d: %v", seed, err)
		}
		if d := res.Div; d != nil {
			divs++
			if mutate {
				return
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			return
		}
	}
	return
}

// TestEachPlayerSearchLockstep plays every carrier's each-player search in
// lockstep and checks Field of Ruin and Veteran Explorer each asked both
// seats at least kesMin times.
func TestEachPlayerSearchLockstep(t *testing.T) {
	t.Parallel()

	games, divs, n := runEachSearch(t, kesSeeds, false)
	keys := make([]string, 0, len(n))
	for k := range n {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var bs []string
	for _, k := range keys {
		bs = append(bs, fmt.Sprintf("%s=%d", k, n[k]))
	}
	t.Logf("%d games, %d divergences; %s", games, divs, strings.Join(bs, " "))
	for _, name := range kesHeadline {
		for s := 0; s < 2; s++ {
			if got := n[fmt.Sprintf("%s/p%d", name, s)]; got < kesMin {
				t.Errorf("%s asked seat %d %d times, want at least %d", name, s, got, kesMin)
			}
		}
	}
}

// TestEachPlayerSearchObjMutation: with every each-player search rewritten to
// the controller's library, some game diverges.
func TestEachPlayerSearchObjMutation(t *testing.T) {
	t.Parallel()

	games, divs, _ := runEachSearch(t, kesSeeds, true)
	if divs == 0 {
		t.Fatalf("mutation in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in game %d", games)
}
