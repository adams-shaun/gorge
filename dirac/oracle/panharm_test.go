package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The Panharmonicon lockstep: the ValidCard$-only statics (Delney's
// "Creature.YouCtrl+powerLE2", Twinflame Travelers' "Elemental.Other+YouCtrl")
// double every triggered ability of the creatures they match; the kernel's cold
// echo must queue the same number of placements as gorge's queueTrigger at every
// decision.

var kpanharmDecks = [][2][]cardset.Entry{{
	// Delney: power-two-or-less enters and dies triggers
	{
		{Name: "Delney, Streetwise Lookout", Count: 4}, {Name: "Burglar Rat", Count: 4}, {Name: "Cloudblazer", Count: 4},
		{Name: "Crow of Dark Tidings", Count: 4}, {Name: "Charming Prince", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Plains", Count: 4},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	// Twinflame Travelers: Elementals with a blocked and a dies trigger
	{
		{Name: "Twinflame Travelers", Count: 4}, {Name: "Ashmouth Hound", Count: 4}, {Name: "Bogardan Firefiend", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Hill Giant", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Island", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Ashmouth Hound", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	// Starfield Vocalist and Virtue of Knowledge (the same static): any
	// permanent entering (a token included) doubles the enters triggers of
	// permanents you control. Virtue also brings Vantress
	// Visions, which copies triggered abilities (a warp delayed trigger
	// included).
	{
		{Name: "Starfield Vocalist", Count: 2}, {Name: "Virtue of Knowledge", Count: 2}, {Name: "Staff of the Storyteller", Count: 2}, {Name: "Burglar Rat", Count: 4},
		{Name: "Crow of Dark Tidings", Count: 4}, {Name: "Welcoming Vampire", Count: 4}, {Name: "Raise the Alarm", Count: 4},
		{Name: "Island", Count: 8}, {Name: "Swamp", Count: 5}, {Name: "Plains", Count: 5},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	// Traveling Chocobo: only a land or Bird you control entering doubles
	// (a Baloths landfall token, a Crow's enters trigger)
	{
		{Name: "Traveling Chocobo", Count: 4}, {Name: "Rampaging Baloths", Count: 4}, {Name: "Crow of Dark Tidings", Count: 4},
		{Name: "Burglar Rat", Count: 4}, {Name: "Forest", Count: 14}, {Name: "Swamp", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	// Delney over keyword triggers: prowess and ward, which queue outside the
	// trigger walk, trigger an additional time like any other (CR 603.2d)
	{
		{Name: "Delney, Streetwise Lookout", Count: 4}, {Name: "Monastery Swiftspear", Count: 8}, {Name: "Armored Armadillo", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 8}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 8},
	},
}}

// runPanharm plays the matchups for seeds 1..seeds and returns the games,
// divergences, stalls, the echoes placed and the games in which a Crow of
// Dark Tidings dies line was echoed.
func runPanharm(t *testing.T, seeds int, mutate bool) (games, divs, stalls, echoes, crowGames int, byMatchup []int) {
	byMatchup = make([]int, len(kpanharmDecks))
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("panharm")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugPanharmNoEcho
	dirac.DebugPanharmNoEcho = mutate
	defer func() { dirac.DebugPanharmNoEcho = old }()
	var log []dirac.PanharmRec
	dirac.PanharmLog = &log
	defer func() { dirac.PanharmLog = nil }()
	for mi, md := range kpanharmDecks {
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
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kpanharm-a", "kpanharm-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := poisonhighPolicy(seed, map[string]int{}, map[string]int{}) // casts whenever it can
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			log = log[:0]
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			echoes += len(log)
			byMatchup[mi] += len(log)
			for _, r := range log {
				if r.Dies && r.Card == "Crow of Dark Tidings" {
					crowGames++
					break
				}
			}
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				if !mutate {
					t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
				}
			}
			if d := res.Div; d != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return
}

func TestPanharmLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, echoes, crowGames, byMatchup := runPanharm(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d echoed placements, %d games with an echoed Crow dies trigger", games, divs, stalls, echoes, crowGames)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	t.Logf("echoed placements by matchup: %v", byMatchup)
	// the ETB-family statics (Starfield Vocalist, Traveling Chocobo)
	for mi := 2; mi < len(byMatchup); mi++ {
		if byMatchup[mi] < 20 {
			t.Errorf("matchup %d: only %d echoed placements (want at least 20)", mi, byMatchup[mi])
		}
	}
	if echoes < 40 {
		t.Errorf("only %d echoed placements (want at least 40)", echoes)
	}
	if crowGames < 5 {
		t.Errorf("only %d games with an echoed Crow of Dark Tidings dies trigger (want at least 5)", crowGames)
	}
}

// TestPanharmMutation switches the echo off: the games must diverge. A
// green mutation (no divergence) means the lockstep never reaches an echo.
func TestPanharmMutation(t *testing.T) {
	games, divs, _, _, _, _ := runPanharm(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation dropped every echo in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
