package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// A spell's own RaiseCost Cost$ Waterbend<N> (Benevolent River Spirit, CR
// 701.67a) against gorge. Gorge's autopay vocabulary offers no waterbend spell
// (its planner declines the shape, "shape:contribution", and so does its
// potential walk; potentialModeCastPlan) except while the floating pool pays it,
// and the vocabulary has no manual mana: so no game casts it, and what the
// lockstep pins is that the kernel offers exactly the casts gorge does -- none of
// Benevolent River Spirit with the credit and the extra {5} priced in -- over
// decks where plain pricing would offer it (the cast and its taps are unit
// tests in dirac/raisecostwaterbend_test.go).

const raiseWaterbendSeeds = 200

func raiseWaterbendDeck() []cardset.Entry {
	return []cardset.Entry{
		{Name: "Benevolent River Spirit", Count: 4}, {Name: "Grizzly Bears", Count: 12},
		{Name: "Mind Stone", Count: 8}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8},
	}
}

func raiseWaterbendRun(t *testing.T, lo, hi uint64, played map[string]int, quiet bool) (diverged, stalls int, stats Stats) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("tlawater")
	if err != nil {
		t.Fatal(err)
	}
	md := [2][]cardset.Entry{raiseWaterbendDeck(), raiseWaterbendDeck()}
	var decks [2][]*cards.Card
	for s, d := range md {
		for _, e := range d {
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
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	why := map[string]int{}
	for seed := lo; seed <= hi; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kraisewb-a", "kraisewb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := kwaterbendPolicy(seed, played)
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		stats.Unknown = addCounts(stats.Unknown, res.Stats.Unknown)
		stats.Unpriced = addCounts(stats.Unpriced, res.Stats.Unpriced)
		stats.UnpricedPlays = addCounts(stats.UnpricedPlays, res.Stats.UnpricedPlays)
		if len(res.Stall) > len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			stalls++
			why[res.Stall]++
			if stalls <= 3 && !quiet {
				t.Logf("seed %d: stall %s at turn %d after %d decisions", seed, res.Stall, res.Turns, res.Decisions)
			}
		}
		if d := res.Div; d != nil {
			diverged++
			if diverged <= 2 && !quiet {
				at := max(d.At-12, 0)
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[at:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("seeds %d-%d: %d diverged, %d kernel stalls %v, plays %v", lo, hi, diverged, stalls, why, played)
	return diverged, stalls, stats
}

func TestRaisecostWaterbendLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	played := map[string]int{}
	diverged, stalls, stats := raiseWaterbendRun(t, 1, raiseWaterbendSeeds, played, false)
	if diverged != 0 {
		t.Errorf("%d games diverged", diverged)
	}
	if stalls != 0 {
		t.Errorf("%d kernel capacity stalls, want zero", stalls)
	}
	// precondition: gorge saw the spell as a potential play it declines, so the
	// offer gate was compared on a hand that held it
	declined := 0
	for k, n := range stats.UnpricedPlays {
		if !strings.HasPrefix(k, "unsupported shape:contribution: Cast Benevolent River Spirit") {
			t.Errorf("Unpriced play beyond the declined waterbend cast: %s", k)
		}
		declined += n
	}
	if declined == 0 {
		t.Error("gorge never listed the Benevolent River Spirit cast as a declined potential play")
	}
	if len(stats.Unknown) != 0 {
		t.Errorf("Unknown options: %v", stats.Unknown)
	}
}
