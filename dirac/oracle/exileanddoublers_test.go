package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The B15/B16 lockstep: constructed matchups over the generated exileanddoublers pool --
// the dies-to-exile replacements (Misery's Shadow, Stone of Erech), the
// filtered counter doublers and plus/minus replacements, and the
// keyword-counter carriers -- so the expected answer at every decision is
// gorge's.

var exileanddoublersDecks = [][2][]cardset.Entry{{
	{
		{Name: "Ozolith, the Shattered Spire", Count: 3}, {Name: "Hardened Scales", Count: 2},
		{Name: "Branching Evolution", Count: 2}, {Name: "Conclave Mentor", Count: 2},
		{Name: "Corpsejack Menace", Count: 2}, {Name: "Kami of Whispered Hopes", Count: 2},
		{Name: "Mowu, Loyal Companion", Count: 2}, {Name: "Vizier of Remedies", Count: 2},
		{Name: "Benevolent Hydra", Count: 2}, {Name: "Michelangelo, Weirdness to 11", Count: 2},
		{Name: "Yoshimaru, Beloved Companion", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Doom Blade", Count: 2}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3},
	},
	{
		{Name: "Misery's Shadow", Count: 4}, {Name: "Stone of Erech", Count: 3},
		{Name: "Mauhúr, Uruk-hai Captain", Count: 2}, {Name: "Walking Corpse", Count: 4},
		{Name: "Vampire Cutthroat", Count: 4}, {Name: "Doom Blade", Count: 3}, {Name: "Murder", Count: 2},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Swamp", Count: 6},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 2},
	},
}, {
	{
		{Name: "Misery's Shadow", Count: 3}, {Name: "Stone of Erech", Count: 3},
		{Name: "Mauhúr, Uruk-hai Captain", Count: 2}, {Name: "Walking Corpse", Count: 4},
		{Name: "Vampire Cutthroat", Count: 4}, {Name: "Doom Blade", Count: 3}, {Name: "Murder", Count: 2},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Swamp", Count: 6},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
	},
	{
		{Name: "Ozolith, the Shattered Spire", Count: 3}, {Name: "Hardened Scales", Count: 2},
		{Name: "Branching Evolution", Count: 2}, {Name: "Conclave Mentor", Count: 2},
		{Name: "Corpsejack Menace", Count: 2}, {Name: "Kami of Whispered Hopes", Count: 2},
		{Name: "Mowu, Loyal Companion", Count: 2}, {Name: "Vizier of Remedies", Count: 2},
		{Name: "Benevolent Hydra", Count: 2}, {Name: "Michelangelo, Weirdness to 11", Count: 2},
		{Name: "Yoshimaru, Beloved Companion", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Doom Blade", Count: 2}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 3}, {Name: "Swamp", Count: 3},
	},
}, {
	// the keyword-counter carriers (Salvation Swan's flying counter,
	// Duskfang Mentor's lifelink, Eagle of Deliverance's indestructible,
	// Void Beckoner's deathtouch, Wingspan Mentor's flying) and Stocking the
	// Pantry's SUPPLY counters, beside the counter-doubling shell and removal.
	{
		{Name: "Salvation Swan", Count: 3}, {Name: "Stocking the Pantry", Count: 2},
		{Name: "Duskfang Mentor", Count: 3}, {Name: "Eagle of Deliverance", Count: 2},
		{Name: "Void Beckoner", Count: 2}, {Name: "Wingspan Mentor", Count: 3},
		{Name: "Hardened Scales", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Doom Blade", Count: 2}, {Name: "Forest", Count: 5}, {Name: "Plains", Count: 6},
		{Name: "Swamp", Count: 3}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Misery's Shadow", Count: 3}, {Name: "Stone of Erech", Count: 3},
		{Name: "Mauhúr, Uruk-hai Captain", Count: 2}, {Name: "Walking Corpse", Count: 4},
		{Name: "Vampire Cutthroat", Count: 4}, {Name: "Doom Blade", Count: 3}, {Name: "Murder", Count: 2},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Swamp", Count: 6},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
	},
}, {
	// The kind filter: a +1/+1-only doubler beside keyword counters. If the
	// doubler ignored its ValidCounterType, Duskfang Mentor's lifelink (and
	// Wingspan Mentor's flying) counters would diverge from gorge.
	{
		{Name: "Duskfang Mentor", Count: 3}, {Name: "Branching Evolution", Count: 3},
		{Name: "Hardened Scales", Count: 2}, {Name: "Salvation Swan", Count: 2},
		{Name: "Wingspan Mentor", Count: 2}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Plains", Count: 6}, {Name: "Swamp", Count: 6},
		{Name: "Island", Count: 4},
	},
	{
		{Name: "Misery's Shadow", Count: 3}, {Name: "Stone of Erech", Count: 3},
		{Name: "Mauhúr, Uruk-hai Captain", Count: 2}, {Name: "Walking Corpse", Count: 4},
		{Name: "Vampire Cutthroat", Count: 4}, {Name: "Doom Blade", Count: 3}, {Name: "Murder", Count: 2},
		{Name: "Gray Ogre", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Swamp", Count: 6},
		{Name: "Plains", Count: 3}, {Name: "Mountain", Count: 3},
	},
}}

// exileanddoublersCarriers names the cards whose casts and activations the test counts,
// with the minimum number of plays each needs: a carrier that never fires
// proves nothing about its mechanism.
var exileanddoublersCarriers = map[string]int{
	// dies-to-exile and the counter-doubling / plus-one shapes.
	"Misery's Shadow": 200, "Stone of Erech": 40,
	"Mauhúr, Uruk-hai Captain":     5,
	"Ozolith, the Shattered Spire": 20, "Hardened Scales": 20, "Branching Evolution": 10,
	"Conclave Mentor": 10, "Corpsejack Menace": 5, "Kami of Whispered Hopes": 10,
	"Mowu, Loyal Companion": 10, "Vizier of Remedies": 10, "Benevolent Hydra": 10,
	"Michelangelo, Weirdness to 11": 10, "Yoshimaru, Beloved Companion": 10,
	// keyword counters granted by a copy and Stocking the Pantry's SUPPLY.
	"Salvation Swan": 5, "Stocking the Pantry": 5, "Duskfang Mentor": 5,
	"Eagle of Deliverance": 2, "Void Beckoner": 2, "Wingspan Mentor": 5,
}

// exileanddoublersSeeds is seeds 1..48 plus games where a one-colour mana source (Kami of
// Whispered Hopes) stands beside a two-colour spell it alone cannot pay.
func exileanddoublersSeeds(matchup int) []uint64 {
	seeds := make([]uint64, 0, 52)
	for seed := uint64(1); seed <= 48; seed++ {
		seeds = append(seeds, seed)
	}
	switch matchup {
	case 0:
		seeds = append(seeds, 117, 370)
	case 1:
		seeds = append(seeds, 209, 325, 413, 490)
	}
	return seeds
}

func TestExileOnDeathAndCounterReplLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("exileanddoublers")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	stalls := map[string]int{}
	for mi, md := range exileanddoublersDecks {
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
		for _, seed := range exileanddoublersSeeds(mi) {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"b1516-a", "b1516-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := castActivatePlayedPolicy(seed, played)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if res.Stall != "" {
				stalls[res.Stall]++
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	t.Logf("%d games, carrier plays %v, stalls %v", games, played, stalls)
	for n, min := range exileanddoublersCarriers {
		if played[n] < min {
			t.Errorf("carrier %s played %d times, want at least %d", n, played[n], min)
		}
	}
}
