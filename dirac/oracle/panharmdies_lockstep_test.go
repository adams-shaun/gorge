package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/testkit/testutil"
)

// The dies-cause Panharmonicon lockstep (Teysa Karlov, Drivnod, Carnage Dominus:
// "If a creature dying causes a triggered ability of a permanent you control to
// trigger, that ability triggers an additional time"; kgen static_triggers.go,
// kop.PanhModeDies, dirac trig_doubling.go panharmDies). Blood Artist and
// Zulaport Cutthroat trigger on creature deaths (a Doomed Traveler's Spirit
// token among them); Burglar Rat, Crow of Dark Tidings and Doomed Traveler have
// their own dies triggers, which the dying permanent's static still doubles.
// The kernel must queue the placements gorge's queueTrigger does.
var kpanharmDiesDecks = [][2][]cardset.Entry{{
	{
		{Name: "Teysa Karlov", Count: 4}, {Name: "Blood Artist", Count: 4}, {Name: "Zulaport Cutthroat", Count: 4},
		{Name: "Doomed Traveler", Count: 4}, {Name: "Burglar Rat", Count: 4}, {Name: "Crow of Dark Tidings", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	{
		{Name: "Drivnod, Carnage Dominus", Count: 4}, {Name: "Blood Artist", Count: 4}, {Name: "Zulaport Cutthroat", Count: 4},
		{Name: "Doomed Traveler", Count: 4}, {Name: "Burglar Rat", Count: 4}, {Name: "Crow of Dark Tidings", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Burglar Rat", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}, {
	// Syr Konrad's ToGraveyard line (a creature card milled or discarded) and its
	// leaves-the-graveyard line (Gravedigger's return) are ChangesZone triggers
	// that a battlefield-to-graveyard move does not cause: gorge's Origin$ gate
	// does not double them under Teysa, while Konrad's own dies line is doubled.
	{
		{Name: "Teysa Karlov", Count: 4}, {Name: "Syr Konrad, the Grim", Count: 4}, {Name: "Gravedigger", Count: 4},
		{Name: "Mind Rot", Count: 4}, {Name: "Burglar Rat", Count: 4}, {Name: "Doomed Traveler", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
	},
	{
		{Name: "Hill Giant", Count: 8}, {Name: "Shock", Count: 4}, {Name: "Mind Rot", Count: 4},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 12},
	},
}}

// runPanharmDies plays the matchups for seeds 1..seeds and returns the games,
// divergences, stalls, the echoed dies-line placements and, per matchup, the
// echoed placements of Blood Artist / Zulaport Cutthroat / Syr Konrad (a creature's death
// that is not their own, which needs no dies line of the source's own).
func runPanharmDies(t *testing.T, seeds int, mutate bool) (games, divs, stalls, diesEchoes, tokenEchoes int, byMatchup []int) {
	t.Helper()
	reg := testutil.CorpusRegistry(t)
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
	byMatchup = make([]int, len(kpanharmDiesDecks))
	for mi, md := range kpanharmDiesDecks {
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
			m, err := NewMatchup(reg, [2]string{"kpanharmdies-a", "kpanharmdies-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			g, err := NewGorgeSide(m, seed, cfg, nil)
			if err != nil {
				t.Fatal(err)
			}
			log = log[:0]
			res := LockstepLed(m, seed, g, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
			CloseSide(k)
			games++
			for _, r := range log {
				if r.Dies {
					diesEchoes++
				}
				if r.Dies && r.Token {
					tokenEchoes++
				}
				if r.Card == "Blood Artist" || r.Card == "Zulaport Cutthroat" || r.Card == "Syr Konrad, the Grim" {
					byMatchup[mi]++
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

func TestPanharmoniconDiesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, diesEchoes, tokenEchoes, byMatchup := runPanharmDies(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, %d echoed dies-line placements (%d caused by a token's death), Blood Artist/Zulaport/Konrad echoes by matchup %v", games, divs, stalls, diesEchoes, tokenEchoes, byMatchup)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	// vacuity: the dies cause must echo a trigger of a creature that is not
	// the one dying, and a dying creature's own dies trigger
	if diesEchoes < 20 {
		t.Errorf("only %d echoed dies-line placements (want at least 20)", diesEchoes)
	}
	if tokenEchoes < 1 {
		t.Errorf("no echoed placement was caused by a token's death (Doomed Traveler's Spirit)")
	}
	for mi, n := range byMatchup {
		if n < 10 {
			t.Errorf("matchup %d: only %d echoed Blood Artist / Zulaport Cutthroat placements (want at least 10)", mi, n)
		}
	}
}

// TestPanharmoniconDiesMutation switches the echo off: the games must diverge.
func TestPanharmoniconDiesMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _, _ := runPanharmDies(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation dropped every echo in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
