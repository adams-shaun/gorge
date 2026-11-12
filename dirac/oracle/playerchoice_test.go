package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated playerchoice pool -- the
// M1/F10 player-choice carriers. Manifold Mouse is the keyword choice
// (KWChoice$), Starseer Mentor / Thornplate Intimidator / Terrapact
// Intimidator the targeted-player GenericChoice (the opponent picks a
// branch at resolution), and Distant Melody / Extinction / Luminescent Rain
// / Witch's Vengeance the mid-resolution ChooseType (Creature.ChosenType
// reads the resolution's answer). The third matchup carries the two other
// ChosenType reads: a filter over non-battlefield cards (Bloodline Shaman,
// For the Ancestors, Elvish Soultiller) and a triggered ability whose source
// is a permanent (Pol Jamaar, Orcrist).
var playerchoiceDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Distant Melody", Count: 3}, {Name: "Luminescent Rain", Count: 3},
			{Name: "Manifold Mouse", Count: 3}, {Name: "Starseer Mentor", Count: 3},
			{Name: "Witch's Vengeance", Count: 3}, {Name: "Doom Blade", Count: 3},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Wind Drake", Count: 2},
			{Name: "Plains", Count: 3}, {Name: "Island", Count: 3},
			{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 3},
			{Name: "Forest", Count: 3},
		},
		{
			{Name: "Extinction", Count: 3}, {Name: "Thornplate Intimidator", Count: 3},
			{Name: "Terrapact Intimidator", Count: 3}, {Name: "Doom Blade", Count: 2},
			{Name: "Shock", Count: 2}, {Name: "Gray Ogre", Count: 3},
			{Name: "Hill Giant", Count: 3}, {Name: "Walking Corpse", Count: 3},
			{Name: "Goblin Piker", Count: 3}, {Name: "Swamp", Count: 8},
			{Name: "Mountain", Count: 7},
		},
	},
	{
		{
			{Name: "Bloodline Shaman", Count: 3}, {Name: "For the Ancestors", Count: 3},
			{Name: "Elvish Soultiller", Count: 3}, {Name: "Pol Jamaar, Illusionist", Count: 3},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Wind Drake", Count: 3},
			{Name: "Doom Blade", Count: 2}, {Name: "Luminescent Rain", Count: 2},
			{Name: "Island", Count: 7}, {Name: "Forest", Count: 10},
		},
		{
			{Name: "Orcrist, Goblin-cleaver", Count: 3}, {Name: "Manifold Mouse", Count: 2},
			{Name: "Thornplate Intimidator", Count: 2}, {Name: "Shock", Count: 3},
			{Name: "Doom Blade", Count: 3}, {Name: "Gray Ogre", Count: 3},
			{Name: "Hill Giant", Count: 3}, {Name: "Goblin Piker", Count: 3},
			{Name: "Walking Corpse", Count: 3}, {Name: "Mountain", Count: 8},
			{Name: "Swamp", Count: 7},
		},
	},
}

// playerchoiceCarriers is the fixed carrier order the test reports in; playerchoiceCarrierMin
// is the minimum number of casts/activations of each the 48-seed games must
// pose (a carrier that never fires proves nothing).
var playerchoiceCarriers = []string{
	"Distant Melody", "Extinction", "Luminescent Rain", "Manifold Mouse",
	"Starseer Mentor", "Terrapact Intimidator", "Thornplate Intimidator",
	"Witch's Vengeance", "Bloodline Shaman", "For the Ancestors",
	"Elvish Soultiller", "Orcrist, Goblin-cleaver", "Pol Jamaar, Illusionist",
}

var playerchoiceCarrierMin = map[string]int{
	"Distant Melody": 10, "Extinction": 10, "Luminescent Rain": 10,
	"Manifold Mouse": 10, "Starseer Mentor": 4, "Terrapact Intimidator": 10,
	"Thornplate Intimidator": 10, "Witch's Vengeance": 8,
	"Bloodline Shaman": 4, "For the Ancestors": 4, "Elvish Soultiller": 2,
	"Orcrist, Goblin-cleaver": 2, "Pol Jamaar, Illusionist": 2,
}

func TestPlayerChoiceCarriersLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("playerchoice")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range playerchoiceDecks {
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
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: true}
			m, err := NewMatchup(reg, [2]string{"b13-a", "b13-b"}, decks, 1)
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
	t.Logf("%d games, carrier plays %v", games, played)
	for _, n := range playerchoiceCarriers {
		if got, min := played[n], playerchoiceCarrierMin[n]; got < min {
			t.Errorf("carrier %s played %d times, want at least %d", n, got, min)
		}
	}
}
