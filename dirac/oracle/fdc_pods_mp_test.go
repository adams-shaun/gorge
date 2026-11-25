package oracle

import (
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kgen"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/deck"
)

// fdcDecks are the five official Foundations Commander (FDC) precons
// (testkit/testutil/decks, pinned by TestFoundationsCommanderDecksAreOfficial).
var fdcDecks = []string{
	"foundations-calling-all-angels",
	"foundations-keen-engineering",
	"foundations-reign-of-dragons",
	"foundations-tramplesaurus-rex",
	"foundations-wretched-ranks",
}

// fdcRefused is every FDC card kgen still refuses: a ratchet that only shrinks.
// A card here plays as Wastes in TestFDCFourSeatPodsLockstep; a card refused
// and not listed, or listed and lowered, fails it.
var fdcRefused = []string{
	"Chandra's Ignition",
	"Cursed Mirror",
	"Cut a Deal",
	"Dragonhawk, Fate's Tempest",
	"Emeria Shepherd",
	"Ezuri's Predation",
	"Goddric, Cloaked Reveler",
	"Hit the Mother Lode",
	"Leyline Tyrant",
	"Sephara, Sky's Blade",
	"Spinerock Knoll",
	"The Elder Dragon War",
	"Thundermane Dragon",
}

// fdcPodSet lowers every card of the five FDC decks into one table, and
// returns the decks with each refused card swapped for Wastes (under the names
// fdc-<deck>), the corpus that deals them and the refused names.
func fdcPodSet(t *testing.T) (cardset.Set, *Corpus, []string, []string) {
	t.Helper()
	base, _ := testCorpus(t)
	c := &Corpus{Reg: base.Reg, DecksDir: filepath.Join("..", "..", "testkit", "testutil", "decks"), decks: map[string][]*cards.Card{}}
	files := map[string]deck.File{}
	seen := map[string]bool{"Wastes": true}
	pool := []string{"Wastes"}
	for _, n := range fdcDecks {
		f, err := c.deckFile(n)
		if err != nil {
			t.Fatal(err)
		}
		files[n] = f
		for _, e := range f.Cards {
			if seen[e.Name] {
				continue
			}
			seen[e.Name] = true
			if _, ok := c.Reg.Lookup(e.Name); !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			pool = append(pool, e.Name)
		}
	}
	// the build a pool's table gets (the corpus's type vocabulary included)
	pd, err := kgen.SynthUnionTable(pool, corpusDirOrSkip(t), "fdc-oracle")
	if err != nil {
		t.Fatal(err)
	}
	set := cardset.AllowUnverified(cardset.Raw(cardset.NewDataSet(pd)))
	sup := set.Supported()
	var refused []string
	out := map[string]deck.File{}
	var names []string
	for _, n := range fdcDecks {
		f := files[n]
		var es []deck.Entry
		sub := 0
		for _, e := range f.Cards {
			if i, ok := set.Lookup(e.Name); ok && sup[i] {
				es = append(es, e)
				continue
			}
			if strings.EqualFold(e.Name, f.Commander) {
				t.Fatalf("%s: the commander %s is refused", n, e.Name)
			}
			if !slices.Contains(refused, e.Name) {
				refused = append(refused, e.Name)
			}
			sub += e.Count
		}
		if sub > 0 {
			es = append(es, deck.Entry{Name: "Wastes", Count: sub})
		}
		f.Cards = es
		out["fdc-"+n] = f
		names = append(names, "fdc-"+n)
	}
	c.AddDecks(out)
	sort.Strings(refused)
	return set, c, names, refused
}

// TestFDCFourSeatPodsLockstep plays the five four-seat pods of the official
// FDC precons (every four of the five decks, full 100-card lists, each seat's
// commander in the command zone), dirac against gorge: no pod diverges or
// stops in the kernel. The refused cards are exactly fdcRefused.
func TestFDCFourSeatPodsLockstep(t *testing.T) {
	set, c, names, refused := fdcPodSet(t)
	want := slices.Clone(fdcRefused)
	sort.Strings(want)
	if !slices.Equal(refused, want) {
		t.Errorf("FDC cards kgen refuses %q, fdcRefused lists %q", refused, want)
	}
	t.Logf("%d FDC cards refused: %q", len(refused), refused)
	for skip := range names {
		pod := slices.Delete(slices.Clone(names), skip, skip+1)
		m, tbl, inter, err := SetMatchupNPlayRefused(set, c, pod, 0)
		if err != nil {
			t.Fatalf("pod without %s: %v", names[skip], err)
		}
		if inter != "" {
			t.Errorf("pod without %s: interaction refused: %s", names[skip], inter)
		}
		if dirac.TwoSeatOnly(tbl) {
			t.Fatalf("pod without %s: the table is two-seat only: %v", names[skip], dirac.TwoSeatCauses(tbl))
		}
		seed := uint64(skip + 1)
		rc := RunConfig{MaxDecisions: 6000, MaxTurns: 40}
		k, err := SetSide(set, tbl, rc.Config)(m, seed)
		if err != nil {
			t.Fatalf("pod without %s: %v", names[skip], err)
		}
		res := Lockstep(m, seed, k, AffordableTargetPolicy(castFirstPolicy(seed)), rc)
		CloseSide(k)
		if d := res.Div; d != nil {
			t.Errorf("pod without %s, seed %d: diverged at decision %d: %s %s %v\nref %q\nker %q\n%s\nrecent %v", names[skip], seed, d.At, d.What, d.Field, d.Diff, d.RefOptions, d.KerOptions, d.Detail, res.RecentSources(m, 4))
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("pod without %s, seed %d: kernel stopped: %s", names[skip], seed, res.Stall)
		}
		t.Logf("pod without %s: %d decisions, stall %q", names[skip], res.Decisions, res.Stall)
	}
}
