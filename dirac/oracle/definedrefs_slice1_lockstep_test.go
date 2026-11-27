package oracle

import (
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// definedRefsSlice1Carriers are the cards this slice turns on: the
// ThisTargetedPlayer, SourceController (player-projection consumers) and
// ParentTargetedController spellings. Glorious End is not here: its
// LosesGame Defined$ SourceController shape stays refused because gorge's
// Defined reads the source object and fails closed, so lowering it would
// diverge (see dirac/kgen/lower_effect_defined.go).
var definedRefsSlice1Carriers = []string{
	"Cybernetica Datasmith", "Depth Defiler", "Prismari Command",
	"Covenant of Minds", "Unlicensed Disintegration", "Season of the Burrow",
}

// definedRefsSlice1Decks are two 40-card constructed decks: every carrier is
// cast from deck A (five-colour mana), while deck B supplies the creatures
// Unlicensed Disintegration and Season of the Burrow target and a second
// player for Cybernetica Datasmith's "another target player".
var definedRefsSlice1Decks = [2][]cardset.Entry{
	{
		{Name: "Unlicensed Disintegration", Count: 3}, {Name: "Season of the Burrow", Count: 3},
		{Name: "Covenant of Minds", Count: 3}, {Name: "Prismari Command", Count: 3},
		{Name: "Cybernetica Datasmith", Count: 3}, {Name: "Depth Defiler", Count: 3},
		{Name: "Mana Confluence", Count: 22},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4},
		{Name: "Plains", Count: 24},
	},
}

// TestDefinedRefsSlice1Lockstep plays the slice's carriers in constructed
// lockstep games, half on each of gorge's autopay surfaces. The cards above
// are not in a standard synthetic pool, so the table is built from the decks'
// own card list. Every carrier must be cast at least once (the precondition),
// and no game may diverge or stall.
func TestDefinedRefsSlice1Lockstep(t *testing.T) {
	t.Parallel()
	seeds := 100 // per surface; 200 games total
	if v := os.Getenv("KDEFINEDREFS_SEEDS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			seeds = n
		}
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	for s, d := range definedRefsSlice1Decks {
		for _, e := range d {
			names[s] = append(names[s], e.Name)
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(definedRefsSlice1Decks[0], definedRefsSlice1Decks[1])
	if err != nil {
		t.Fatal(err)
	}
	var gd [2][]*cards.Card
	for s, d := range definedRefsSlice1Decks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				gd[s] = append(gd[s], c)
			}
		}
		if len(gd[s]) != 40 {
			t.Fatalf("deck %d: %d cards, want 40", s, len(gd[s]))
		}
	}
	watch := make(map[string]bool, len(definedRefsSlice1Carriers))
	for _, n := range definedRefsSlice1Carriers {
		watch[n] = true
	}
	casts := map[string]int{}
	for _, surface := range []bool{false, true} {
		cfg := Config{GorgeAutopaySurface: surface}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			m, err := NewMatchup(reg, [2]string{"definedrefs-a", "definedrefs-b"}, gd, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := NewGorgeSide(m, seed, cfg, nil); err != nil {
				CloseSide(k)
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, fixtureWatchPolicy(seed, watch, casts), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			if res.Div != nil {
				t.Fatalf("autopay=%v seed %d: diverged at decision %d: %s %s %v\nref %v\nker %v",
					surface, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Fatalf("autopay=%v seed %d: kernel stall %q", surface, seed, res.Stall)
			}
		}
	}
	for _, n := range definedRefsSlice1Carriers {
		if casts[n] == 0 {
			t.Errorf("%s was never cast across the games: the fixture does not exercise it", n)
		}
	}
	t.Logf("defined refs slice 1: %d games per surface; casts %v", seeds, casts)
}
