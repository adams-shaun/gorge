package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Holodeck lockstep: "Once during each of your turns, you may play a
// permanent card you own from outside the game or from your graveyard" (the
// static's AffectedZone$ Sideboard,Graveyard, CR 400.11). Each seat carries a
// real sideboard in both engines, and the kernel must offer the same sideboard
// and graveyard plays, and spend the turn's one permission the same way, at
// every decision.

// kholodeck is one matchup: the two decks and the two sideboards.
type kholodeck struct {
	deck [2][]cardset.Entry
	side [2][]cardset.Entry
}

var kholodeckMatchups = []kholodeck{{
	// seat 0 ramps into The Holodeck; seat 1 kills its creatures (so the
	// graveyard holds permanent cards) and has a Holodeck of its own
	deck: [2][]cardset.Entry{{
		{Name: "The Holodeck", Count: 4}, {Name: "Sol Ring", Count: 3}, {Name: "Llanowar Elves", Count: 5},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Forest", Count: 16}, {Name: "Swamp", Count: 8},
	}, {
		{Name: "Shock", Count: 6}, {Name: "Doom Blade", Count: 4}, {Name: "Savannah Lions", Count: 6},
		{Name: "The Holodeck", Count: 2}, {Name: "Sol Ring", Count: 2}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	}},
	side: [2][]cardset.Entry{{
		{Name: "Grizzly Bears", Count: 3}, {Name: "Forest", Count: 3}, {Name: "Sol Ring", Count: 1},
		{Name: "Llanowar Elves", Count: 2}, {Name: "Savannah Lions", Count: 2},
	}, {
		{Name: "Sol Ring", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 2},
	}},
}, {
	// a thin library of lands: a land play from the sideboard (the land drop
	// and the turn's permission) beside a land in the graveyard
	deck: [2][]cardset.Entry{{
		{Name: "The Holodeck", Count: 6}, {Name: "Sol Ring", Count: 4}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Forest", Count: 14}, {Name: "Swamp", Count: 10},
	}, {
		{Name: "Doom Blade", Count: 6}, {Name: "Shock", Count: 6}, {Name: "Savannah Lions", Count: 8},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	}},
	side: [2][]cardset.Entry{{
		{Name: "Forest", Count: 6}, {Name: "Swamp", Count: 3}, {Name: "Grizzly Bears", Count: 3},
	}, {
		{Name: "Plains", Count: 3},
	}},
}}

// runHolodeck plays the matchups for seeds 1..seeds and returns the games,
// divergences, stalls and the games in which a seat played a card out of its
// sideboard.
func runHolodeck(t *testing.T, seeds int, mutate bool) (games, divs, stalls, sidePlays int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("holodeck")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugHolodeckNoSideboard
	dirac.DebugHolodeckNoSideboard = mutate
	defer func() { dirac.DebugHolodeckNoSideboard = old }()
	for mi, md := range kholodeckMatchups {
		var decks, sides [2][]*cards.Card
		for s := range md.deck {
			decks[s] = expandEntries(t, reg, md.deck[s])
			sides[s] = expandEntries(t, reg, md.side[s])
			if len(decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(append(append([]cardset.Entry(nil), md.deck[0]...), md.side[0]...), append(append([]cardset.Entry(nil), md.deck[1]...), md.side[1]...))
		if err != nil {
			t.Fatal(err)
		}
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchupSide(reg, [2]string{"kholodeck-a", "kholodeck-b"}, decks, sides, 1)
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
			res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
			CloseSide(k)
			games++
			for p := range sides {
				if len(g.E.G.Zone(state.ZSideboard, state.PlayerID(p))) < len(sides[p]) {
					sidePlays++
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

func TestTheHolodeckLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, sidePlays := runHolodeck(t, 128, false)
	t.Logf("%d games, %d divergences, %d stalls, %d games with a card played from a sideboard", games, divs, stalls, sidePlays)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if sidePlays < 20 {
		t.Errorf("only %d games played a card from a sideboard (want at least 20)", sidePlays)
	}
}

// TestTheHolodeckMutation switches the kernel's sideboard offer off: the games
// must diverge. A green mutation (no divergence) means the lockstep never
// reaches a sideboard play.
func TestTheHolodeckMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _ := runHolodeck(t, 32, true)
	if divs == 0 {
		t.Fatalf("mutation dropped every sideboard offer in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
