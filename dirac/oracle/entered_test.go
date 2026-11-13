package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated
// entered pool. The carriers read every shape the generic
// Count$ThisTurnEntered head lowers to:
//
//   - dest Battlefield, live permanent and card form (Kinbinding,
//     Waterspout Warden, Malcator, Bespoke Battlegarb, Bioengineered Future,
//     Motivated Pony);
//   - dest Graveyard/Exile from the battlefield with the departure-time
//     creature split (Bontu, Ichor Shade, Vren, Kami of Transience,
//     Ashen-Skin Zubera);
//   - dest Hand from the battlefield (Barrin);
//   - dest Graveyard with no origin (Macabre Reconstruction, Welcome the
//     Dead);
//   - dest Battlefield with Other/StrictlyOther (Waterspout Warden, White
//     Glove Gourmand).
//
// Cloudshift makes a creature enter twice; Pyroclasm/Naturalize/Bake into a
// Pie move an object on after it entered; Bontu and the token makers make
// tokens cease. The frozen FDN and BLB pair pools exercise none of these
// arms.

var kenteredDecks = [][2][]cardset.Entry{{
	{
		{Name: "Kinbinding", Count: 2}, {Name: "Waterspout Warden", Count: 2},
		{Name: "Cloudshift", Count: 2}, {Name: "Barrin, Tolarian Archmage", Count: 2},
		{Name: "Malcator, Purity Overseer", Count: 2}, {Name: "Kami of Transience", Count: 2},
		{Name: "White Glove Gourmand", Count: 2}, {Name: "Bioengineered Future", Count: 2},
		{Name: "Motivated Pony", Count: 1}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Plains", Count: 5}, {Name: "Island", Count: 5}, {Name: "Forest", Count: 4},
		{Name: "Swamp", Count: 3}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Bontu the Glorified", Count: 2}, {Name: "Ashen-Skin Zubera", Count: 2},
		{Name: "Welcome the Dead", Count: 1}, {Name: "Macabre Reconstruction", Count: 1},
		{Name: "Ichor Shade", Count: 2}, {Name: "Vren, the Relentless", Count: 2},
		{Name: "Bespoke Battlegarb", Count: 2}, {Name: "Naturalize", Count: 2},
		{Name: "Bake into a Pie", Count: 2}, {Name: "Thought Scour", Count: 1},
		{Name: "Unsummon", Count: 1}, {Name: "Pyroclasm", Count: 1},
		{Name: "Walking Corpse", Count: 1},
		{Name: "Swamp", Count: 5}, {Name: "Forest", Count: 5}, {Name: "Mountain", Count: 4},
		{Name: "Island", Count: 3}, {Name: "Plains", Count: 3},
	},
}}

// kenteredCarriers names the cards whose casts and activations the test
// counts (each must be played at least once).
var kenteredCarriers = []string{
	"Kinbinding", "Waterspout Warden", "Barrin, Tolarian Archmage",
	"Malcator, Purity Overseer", "Kami of Transience", "White Glove Gourmand",
	"Bioengineered Future", "Motivated Pony", "Bontu the Glorified",
	"Ashen-Skin Zubera", "Welcome the Dead", "Macabre Reconstruction",
	"Ichor Shade", "Vren, the Relentless", "Bespoke Battlegarb",
	"Cloudshift", "Pyroclasm", "Bake into a Pie", "Naturalize", "Unsummon",
}

func TestThisTurnEnteredLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("entered")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	matchupChunks(t, len(kenteredDecks), 1, 1024, 128, func(t *testing.T, c seedChunk, mu *sync.Mutex) {
		mi, md := c.mi, kenteredDecks[c.mi]
		var decks [2][]*cards.Card
		for s, d := range md {
			for _, e := range d {
				card, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("matchup %d: %s not in the corpus", mi, e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[s] = append(decks[s], card)
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
		n, part := 0, map[string]int{}
		defer func() {
			mu.Lock()
			defer mu.Unlock()
			games += n
			for k, v := range part {
				played[k] += v
			}
		}()
		for seed := c.lo; seed <= c.hi; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kentered-a", "kentered-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := eagerPolicy(seed, part)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			n++
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				return
			}
		}
	})
	if t.Failed() {
		return
	}
	t.Logf("%d games, carrier plays %v", games, played)
	for _, n := range kenteredCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
}

// TestThisTurnEnteredMutation flips the debug switch that reverts
// thisTurnEntered to the refused zoneTurn reader: an object counts only if
// its CURRENT zone is the destination, so a double entry counts once and a
// card that entered and moved on is lost. The lockstep must diverge.
func TestThisTurnEnteredMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("entered")
	if err != nil {
		t.Fatal(err)
	}
	md := kenteredDecks[0]
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
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugEnteredLive
	dirac.DebugEnteredLive = true
	defer func() { dirac.DebugEnteredLive = old }()
	for seed := uint64(1); seed <= 64; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kentered-a", "kentered-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, eagerPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			t.Logf("mutation diverged at seed %d: %s %s %v", seed, res.Div.What, res.Div.Field, res.Div.Diff)
			return
		}
	}
	t.Fatal("debug switch did not diverge in 64 seeds")
}
