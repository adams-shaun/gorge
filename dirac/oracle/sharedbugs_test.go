package oracle

import (
	"os"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
)

// sharedBugsDecks are small boards built to reach what gorge and the kernel once
// agreed on against the Comprehensive Rules. Each side plays the same 40 cards.
var sharedBugsDecks = []struct {
	name string
	deck []cardset.Entry
}{
	{"untap order and stun", []cardset.Entry{{Name: "Sleep Magic", Count: 12}, {Name: "Hill Giant", Count: 4}, {Name: "Slumbering Cerberus", Count: 4},
		{Name: "Sleep-Cursed Faerie", Count: 4}, {Name: "Island", Count: 8}, {Name: "Mountain", Count: 8}}},
	{"left under your control", []cardset.Entry{{Name: "Act of Treason", Count: 8}, {Name: "Viscera Seer", Count: 4}, {Name: "Foot Mystic", Count: 4},
		{Name: "Grizzly Bears", Count: 8}, {Name: "Swamp", Count: 8}, {Name: "Mountain", Count: 8}}},
	{"madness discard", []cardset.Entry{{Name: "Tormenting Voice", Count: 10}, {Name: "Fiery Temper", Count: 8}, {Name: "Marauding Mako", Count: 6},
		{Name: "Island", Count: 8}, {Name: "Mountain", Count: 8}}},
	{"opposite counters", []cardset.Entry{{Name: "Meltstrider Eulogist", Count: 6}, {Name: "Incremental Blight", Count: 6},
		{Name: "Battlegrowth", Count: 8}, {Name: "Grizzly Bears", Count: 6}, {Name: "Swamp", Count: 7}, {Name: "Forest", Count: 7}}},
	{"entering power", []cardset.Entry{{Name: "Glorious Anthem", Count: 6}, {Name: "Railway Brawler", Count: 6},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Lightning Bolt", Count: 4}, {Name: "Mountain", Count: 9}, {Name: "Plains", Count: 5}, {Name: "Forest", Count: 4}}},
	{"cast trigger reads X", []cardset.Entry{{Name: "Kaervek the Merciless", Count: 6}, {Name: "Blaze", Count: 8}, {Name: "Goblin Electromancer", Count: 6},
		{Name: "Mountain", Count: 12}, {Name: "Swamp", Count: 4}, {Name: "Island", Count: 4}}},
	{"cast trigger reads the face", []cardset.Entry{{Name: "Manaplasm", Count: 8}, {Name: "Fire", Count: 8}, {Name: "Mind Spring", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 4}, {Name: "Forest", Count: 4}}},
	{"departed conniver and explorer", []cardset.Entry{{Name: "Red Room Recruit", Count: 8}, {Name: "Kinjalli's Dawnrunner", Count: 8}, {Name: "Lightning Bolt", Count: 8},
		{Name: "Mountain", Count: 8}, {Name: "Plains", Count: 8}}},
	{"suspect and control", []cardset.Entry{{Name: "Barbed Servitor", Count: 10}, {Name: "Act of Treason", Count: 10}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 16}}},
	{"mayhem object", []cardset.Entry{{Name: "Electro's Bolt", Count: 12}, {Name: "Tormenting Voice", Count: 12}, {Name: "Hill Giant", Count: 2}, {Name: "Mountain", Count: 14}}},
	{"modal activation", []cardset.Entry{{Name: "Phoenix Down", Count: 6}, {Name: "Insidious Fungus", Count: 2}, {Name: "Bow of Nylea", Count: 4}, {Name: "Grizzly Bears", Count: 6}, {Name: "Lightning Bolt", Count: 6},
		{Name: "Mountain", Count: 6}, {Name: "Plains", Count: 6}, {Name: "Forest", Count: 4}}},
	{"spend of a departed creature", []cardset.Entry{{Name: "Astelli Reclaimer", Count: 10}, {Name: "Lightning Bolt", Count: 8}, {Name: "Grizzly Bears", Count: 4}, {Name: "Mountain", Count: 10}, {Name: "Plains", Count: 8}}},
	{"bargained target", []cardset.Entry{{Name: "Brave the Wilds", Count: 12}, {Name: "Ornithopter", Count: 10}, {Name: "Forest", Count: 18}}},
	{"effect on several permanents", []cardset.Entry{{Name: "Secret Tunnel", Count: 8}, {Name: "Lightning Bolt", Count: 6}, {Name: "Grizzly Bears", Count: 10}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 8}}},
	{"spell on the stack", []cardset.Entry{{Name: "Quandrix, the Proof", Count: 8}, {Name: "Opt", Count: 12}, {Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 8}}},
}

// TestSharedBugsLockstep plays the boards against gorge on both surfaces.
func TestSharedBugsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, _ := kopusSetup(t)
	set, err := cardset.RawByName("sharedbugs")
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range sharedBugsDecks {
		t.Run(strings.ReplaceAll(row.name, " ", "_"), func(t *testing.T) {
			md := [2][]cardset.Entry{row.deck, row.deck}
			decks := kcastDeck(t, reg, md)
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			divs := 0
			for seed := uint64(1); seed <= 200; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"sb-a", "sb-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				pol := Policy(HeuristicPolicy(seed))
				if seed%3 == 0 {
					pol = RandomPolicy(seed)
				}
				res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30})
				CloseSide(k)
				if res.StallUnmodelled {
					t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
				}
				if res.Div != nil {
					if divs++; divs <= 2 {
						t.Errorf("seed %d: diverged at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
						if os.Getenv("SB_DUMP") != "" {
							t.Logf("actions %v\nref %v\nker %v", actionStrings(res.Actions[max(0, res.Div.At-14):res.Div.At]), res.Div.RefOptions, res.Div.KerOptions)
						}
					}
				}
			}
			if divs > 2 {
				t.Errorf("%d of 200 games diverged", divs)
			}
		})
	}
}

// TestPanharmonicBlockLines: gorge doubles a block line like any other trigger, and
// so does the kernel's echo (CR 603.2d). The boards must reach the echo.
func TestPanharmonicBlockLines(t *testing.T) {
	if isolate(t) {
		return
	}
	reg, _ := kopusSetup(t)
	set, err := cardset.RawByName("sharedbugs")
	if err != nil {
		t.Fatal(err)
	}
	deck := []cardset.Entry{{Name: "Delney, Streetwise Lookout", Count: 8}, {Name: "Alley Grifters", Count: 8}, {Name: "Town Sentry", Count: 8},
		{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8}}
	decks := kcastDeck(t, reg, [2][]cardset.Entry{deck, deck})
	tbl, _, _, err := set.Matchup(deck, deck)
	if err != nil {
		t.Fatal(err)
	}
	var log []dirac.PanharmRec
	dirac.PanharmLog = &log
	defer func() { dirac.PanharmLog = nil }()
	for seed := uint64(1); seed <= 120; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"sb-a", "sb-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, HeuristicPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: diverged at %d: %s %s %v %s", seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail)
		}
	}
	n := 0
	for _, r := range log {
		if r.Card == "Alley Grifters" || r.Card == "Town Sentry" {
			n++
		}
	}
	if n < 10 {
		t.Fatalf("%d echoed block lines, want at least 10", n)
	}
	t.Logf("%d echoed block lines", n)
}
