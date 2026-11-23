package oracle

import (
	"os"
	"strconv"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// TestUnattachEffectLockstep: api:Unattach (gorge engine/effects/unattach.go
// effUnattach) in lockstep over the unattacheffect fixture pool. Carry Away's
// enter trigger unattaches the Equipment it enchants; Captain's Hook, Grafted
// Wargear and Stitcher's Graft print a Mode$ Unattached trigger, so a detach
// the effect makes must publish the same trigger gorge does (the former
// bearer rides the event). Both payment surfaces run (every fourth seed uses
// gorge's autopay surface). The finish hook counts the effect's own detaches
// off gorge's finished event log, the Unattached events with no arm text
// (the attachment-SBA and re-equip arms carry one), and the test fails when
// the pool never reaches one, so it cannot pass vacuously.
//
// dirac is two-seat, so there is no pod arm.
var kunattEffDecks = [2][]cardset.Entry{
	{
		{Name: "Carry Away", Count: 4}, {Name: "Captain's Hook", Count: 3}, {Name: "Grafted Wargear", Count: 3}, {Name: "Stitcher's Graft", Count: 2},
		{Name: "Memnite", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 4}, {Name: "Hill Giant", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Disenchant", Count: 1},
		{Name: "Island", Count: 7}, {Name: "Forest", Count: 3}, {Name: "Mountain", Count: 1},
	},
	{
		{Name: "Carry Away", Count: 3}, {Name: "Captain's Hook", Count: 3}, {Name: "Grafted Wargear", Count: 3}, {Name: "Stitcher's Graft", Count: 3},
		{Name: "Memnite", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 2}, {Name: "Hill Giant", Count: 2},
		{Name: "Lightning Bolt", Count: 2}, {Name: "Disenchant", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Mountain", Count: 5}, {Name: "Plains", Count: 1},
	},
}

func TestUnattachEffectLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unattacheffect")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kunattEffDecks)
	tbl, _, _, err := set.Matchup(kunattEffDecks[0], kunattEffDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	seeds := uint64(96)
	if v, e := strconv.Atoi(os.Getenv("UNATT_EFFECT_SEEDS")); e == nil && v > 0 {
		seeds = uint64(v)
	}
	effect, publishes := 0, 0
	finish := func(g *GorgeSide) {
		for i := range g.E.L.Events {
			if e := &g.E.L.Events[i]; e.Kind == events.Unattached {
				publishes++
				if e.Text == "" {
					effect++
				}
			}
		}
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{}
		if seed%4 == 0 {
			cfg = Config{GorgeAutopaySurface: true}
		}
		m, err := NewMatchup(reg, [2]string{"kunatteff-a", "kunatteff-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kunattPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25, Finish: finish})
		CloseSide(k)
		if s := res.Stall; len(s) >= len(KernelStallPrefix) && s[:len(KernelStallPrefix)] == KernelStallPrefix {
			t.Fatalf("seed %d: kernel stall %s", seed, s)
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d seeds: %d Unattached publishes, %d from the effect", seeds, publishes, effect)
	if effect < 3 {
		t.Fatalf("only %d effect detaches over %d seeds: the pool does not exercise api:Unattach", effect, seeds)
	}
}

// TestUnattachEffectMutationDiverges is the pool's mutation check:
// DebugUnattachNoDetach resolves the effect without detaching, so a game
// that unattaches must diverge from gorge.
func TestUnattachEffectMutationDiverges(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unattacheffect")
	if err != nil {
		t.Fatal(err)
	}
	decks := kcastDeck(t, reg, kunattEffDecks)
	tbl, _, _, err := set.Matchup(kunattEffDecks[0], kunattEffDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	dirac.DebugUnattachNoDetach = true
	defer func() { dirac.DebugUnattachNoDetach = false }()
	for seed := uint64(1); seed <= 96; seed++ {
		m, err := NewMatchup(reg, [2]string{"kunatteff-a", "kunatteff-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, Config{})(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kunattPolicy(seed), RunConfig{MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		if res.Div != nil {
			return
		}
	}
	t.Fatal("DebugUnattachNoDetach never diverged over 96 seeds: the pool does not observe the detach")
}
