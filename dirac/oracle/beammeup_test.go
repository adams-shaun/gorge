package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The beam me up lockstep: Open Communications cast from the graveyard, with a
// creature returned and the spell exiled after, and Marooned barring the return.
var kbeamDecks = [2][]cardset.Entry{
	{
		{Name: "Open Communications", Count: 8}, {Name: "Burning-Tree Emissary", Count: 6}, {Name: "Lotus Cobra", Count: 4},
		{Name: "Mardu Warshrieker", Count: 2}, {Name: "Marooned", Count: 2}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Open Communications", Count: 8}, {Name: "Burning-Tree Emissary", Count: 6}, {Name: "Lotus Cobra", Count: 4},
		{Name: "Mardu Warshrieker", Count: 2}, {Name: "Marooned", Count: 2}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 6},
	},
}

// kbeamPolicy casts a beam me up spell whenever offered and otherwise answers at
// random. gorge's planner prices no beam me up cast, so it is offered only while the
// floating pool pays: Burning-Tree Emissary's mana with Lotus Cobra's. beams counts
// the casts and returns the return picks.
func kbeamPolicy(seed uint64, beams, returns, both *int) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if d := c.G.E.Pending(); d != nil {
			if d.Kind == decision.KChoose && len(d.Options) > 0 && d.Options[0].Kind == "returncost" {
				*returns++
			}
			if d.Kind == decision.KPriority {
				var pick Canon
				n := 0
				for _, a := range c.D.Options {
					if a.Kind() != ActCast || a.Mode()&15 != castModeOther {
						continue
					}
					if n == 0 {
						pick = a
					}
					n++
					// the card's second permission, the grant: taken on odd seeds
					if a.Mode()>>4 == 1 {
						*both++
						if seed%2 == 1 {
							pick = a
						}
					}
				}
				if n > 0 {
					*beams++
					return pick
				}
			}
		}
		return rnd.Pick(c)
	})
}

// kbeamGrantDecks: Scotty's grant over plain spells and a printed flashback, beside
// Scotty himself as the only creature the grant cannot return.
var kbeamGrantDecks = [2][]cardset.Entry{
	{
		{Name: "Scotty, Miracle Worker", Count: 8}, {Name: "Think Twice", Count: 6}, {Name: "Opt", Count: 6},
		{Name: "Burning-Tree Emissary", Count: 6}, {Name: "Lotus Cobra", Count: 4}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Scotty, Miracle Worker", Count: 8}, {Name: "Think Twice", Count: 6}, {Name: "Opt", Count: 6},
		{Name: "Burning-Tree Emissary", Count: 6}, {Name: "Lotus Cobra", Count: 4}, {Name: "Island", Count: 6}, {Name: "Forest", Count: 4},
	},
}

// kbeamFlashbackDecks: Sphinx of Forgotten Lore's flashback grant on a card with
// printed beam me up, which then offers both casts.
var kbeamFlashbackDecks = [2][]cardset.Entry{
	{
		{Name: "Sphinx of Forgotten Lore", Count: 10}, {Name: "Open Communications", Count: 8}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Lotus Cobra", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Sphinx of Forgotten Lore", Count: 10}, {Name: "Open Communications", Count: 8}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Lotus Cobra", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 4},
	},
}

// kbeamBesideDecks: Scotty's grant over a card that prints beam me up, which then
// has two casts, {2}{U} printed and {U} granted, told apart by the alt-cost nibble.
var kbeamBesideDecks = [2][]cardset.Entry{
	{
		{Name: "Scotty, Miracle Worker", Count: 8}, {Name: "Open Communications", Count: 8}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Lotus Cobra", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 2},
	},
	{
		{Name: "Scotty, Miracle Worker", Count: 8}, {Name: "Open Communications", Count: 8}, {Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Lotus Cobra", Count: 4}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 4}, {Name: "Mountain", Count: 2},
	},
}

func TestBeamMeUpLockstep(t *testing.T) { kbeamLockstep(t, kbeamDecks, 0) }

// TestBeamBesideGrantLockstep: a printed beam me up card beside a grant offers both
// casts. The policy takes the granted one on odd seeds and the printed on even, so
// each is played, and both must be offered together at least once.
func TestBeamBesideGrantLockstep(t *testing.T) { kbeamLockstep(t, kbeamBesideDecks, 20) }

func TestBeamGrantLockstep(t *testing.T) { kbeamLockstep(t, kbeamGrantDecks, 0) }

func TestBeamBesideFlashbackGrantLockstep(t *testing.T) {
	kbeamLockstep(t, kbeamFlashbackDecks, 0)
}

// kbeamLockstep plays the decks in lockstep; wantBoth is the fewest windows that
// must offer a card's second beam me up permission (the grant beside the printed).
func kbeamLockstep(t *testing.T, kdecks [2][]cardset.Entry, wantBoth int) {
	if isolate(t) {
		return
	}
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("beammeup")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kdecks {
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
	tbl, _, _, err := set.Matchup(kdecks[0], kdecks[1])
	if err != nil {
		t.Fatal(err)
	}
	beams, returns, both := 0, 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kbeam-a", "kbeam-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kbeamPolicy(seed, &beams, &returns, &both), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 120})
		CloseSide(k)
		if res.Stall != "" {
			t.Fatalf("seed %d: stall %s after %d decisions", seed, res.Stall, res.Decisions)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
				seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d beam casts, %d return picks, %d second permissions offered", beams, returns, both)
	if beams < 5 || returns < 5 {
		t.Errorf("thin coverage: %d beam casts, %d return picks", beams, returns)
	}
	if both < wantBoth {
		t.Errorf("%d windows offered a card's second beam me up permission, want %d", both, wantBoth)
	}
}
