package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Any-colour TapsForMana extras (Fertile Ground, Dawn's Reflection, Market
// Festival and their kin) played decision by decision against gorge. The
// trigger asks a colour at the tap (CR 605.3b), which gorge's manual window
// answers deterministically; the kernel's deferred tap floats the ask's
// answer at the tap itself (dirac/mana_tapextra.go addGatedExtra). The cards
// are played one at a time: a deck mixing two of the Auras exercises a
// separate tap-deferral interaction, not this lowering. Trace of Abundance is
// lowered and its census row drops, but its hybrid {R/W} cost needs the
// announced half in the extra's answer, a separate defect (the follow-up
// ticket).
var kanyColCards = []string{
	"Blighted Burgeoning", "Buried in the Garden", "Dawn's Reflection",
	"Fertile Ground", "Market Festival", "Verdant Haven",
}

// kanyColDecks is one card's 40-card deck: four copies of the Aura, basics of
// every type to enchant and tap, and cheap spells of each colour so the
// trigger's colour ask funds real casts.
func kanyColDecks(name string) [2][]cardset.Entry {
	return [2][]cardset.Entry{
		{
			{Name: name, Count: 4}, {Name: "Grizzly Bears", Count: 6},
			{Name: "Forest", Count: 6}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 4},
			{Name: "Mountain", Count: 4}, {Name: "Swamp", Count: 4},
			{Name: "Opt", Count: 4}, {Name: "Shock", Count: 4},
		},
		{
			{Name: "Grizzly Bears", Count: 12}, {Name: "Shock", Count: 8},
			{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
		},
	}
}

// TestTapsForManaTwoSeatLockstep plays each any-colour extra's games at two
// seats: no divergence and no stall.
func TestTapsForManaTwoSeatLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range kanyColCards {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if n := lockstepRun(t, "tapsformana", kanyColDecks(name), 100, false, nil); n != 0 {
				t.Fatalf("%d divergent seeds", n)
			}
		})
	}
}

// TestTapsForManaFourSeatLockstep is the four-seat form of the same games,
// which exercises the extra's ask across four payers and their lands.
func TestTapsForManaFourSeatLockstep(t *testing.T) {
	t.Parallel()
	for _, name := range kanyColCards {
		name := name
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			anyColFourSeat(t, kanyColDecks(name)[0], 24, nil)
		})
	}
}

// TestTapsForManaColourAskMutation: with the any-colour extra dropped at the
// tap (dirac.DebugTapAnyColOff), some seed diverges, so the locksteps exercise
// the ask rather than passing on pools that never tap an enchanted land.
func TestTapsForManaColourAskMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	decks := kanyColDecks("Fertile Ground")
	dirac.DebugTapAnyColOff = true
	defer func() { dirac.DebugTapAnyColOff = false }()
	if lockstepRun(t, "tapsformana", decks, 100, false, nil) == 0 {
		t.Fatal("no seed diverged with the any-colour tap extra dropped: the pool does not exercise the ask")
	}
}

// anyColFourSeat plays one any-colour deck at four seats, failing on a
// divergence or a stall.
func anyColFourSeat(t *testing.T, entries []cardset.Entry, seeds uint64, mutate func(*kop.Table)) {
	t.Helper()
	reg := synthReg(t)
	set, err := cardset.RawByName("tapsformana")
	if err != nil {
		t.Fatal(err)
	}
	var deck []*cards.Card
	for _, e := range entries {
		c, ok := reg.Lookup(e.Name)
		if !ok {
			t.Fatalf("%s not in the corpus", e.Name)
		}
		for i := 0; i < e.Count; i++ {
			deck = append(deck, c)
		}
	}
	if len(deck) != 40 {
		t.Fatalf("deck: %d cards", len(deck))
	}
	lists := [][]cardset.Entry{entries, entries, entries, entries}
	tbl, _, err := set.MatchupN(lists)
	if err != nil {
		t.Fatal(err)
	}
	if mutate != nil {
		mutate(tbl)
	}
	for seed := uint64(1); seed <= seeds; seed++ {
		m, err := NewMatchupN(reg, []string{"a", "b", "c", "d"},
			[][]*cards.Card{deck, deck, deck, deck}, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{}
		if seed%2 == 0 {
			cfg.GorgeAutopaySurface = true
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, castFirstPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 8000, MaxTurns: 100})
		CloseSide(k)
		if res.Div != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v\nref %v\nker %v",
				seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.RefOptions, res.Div.KerOptions)
		}
		if res.StallUnmodelled || strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Fatalf("seed %d: kernel stopped: %s", seed, res.Stall)
		}
	}
}
