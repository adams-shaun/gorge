package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The all-of type predicate lockstep (typesall): constructed matchups over
// the typesall pool, whose "Artifact.Creature" filters (Steel Overseer's
// counters, Tempered Steel's, Master of Etherium's and Chief of the
// Foundry's anthems, Thopter Engineer's haste grant, Pia Nalaar's Thopters)
// and "Land.Artifact" search (Sojourner's Companion's artifact landcycling)
// lower to PredTypesAll, played beside artifact creatures, nonartifact
// creatures, noncreature artifacts and artifact lands on both sides, and
// Ensoul Artifact (an animated noncreature artifact is an artifact creature:
// the derived-type read on the permanent path). The expected answer at
// every decision is gorge's.
var ktypesallDecks = [2][]cardset.Entry{
	{
		{Name: "Steel Overseer", Count: 3}, {Name: "Tempered Steel", Count: 2}, {Name: "Master of Etherium", Count: 2},
		{Name: "Chief of the Foundry", Count: 2}, {Name: "Ornithopter", Count: 3}, {Name: "Alloy Myr", Count: 2},
		{Name: "Gold Myr", Count: 2}, {Name: "Mind Stone", Count: 2}, {Name: "Ensoul Artifact", Count: 3},
		{Name: "Savannah Lions", Count: 2}, {Name: "Coral Merfolk", Count: 2},
		{Name: "Seat of the Synod", Count: 4}, {Name: "Ancient Den", Count: 4}, {Name: "Plains", Count: 4}, {Name: "Island", Count: 3},
	},
	{
		{Name: "Thopter Engineer", Count: 3}, {Name: "Pia Nalaar", Count: 2}, {Name: "Sojourner's Companion", Count: 3},
		{Name: "Juggernaut", Count: 2}, {Name: "Alloy Myr", Count: 2}, {Name: "Mind Stone", Count: 1},
		{Name: "Shatter", Count: 2}, {Name: "Naturalize", Count: 1}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Hill Giant", Count: 2}, {Name: "Wall of Tanglecord", Count: 2},
		{Name: "Great Furnace", Count: 4}, {Name: "Tree of Tales", Count: 3}, {Name: "Darksteel Citadel", Count: 2},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// ktypesallCarriers are the pool's PredTypesAll carriers (their casts and
// activations are counted).
var ktypesallCarriers = map[string]bool{"Steel Overseer": true, "Tempered Steel": true, "Master of Etherium": true,
	"Chief of the Foundry": true, "Thopter Engineer": true, "Pia Nalaar": true, "Sojourner's Companion": true}

func TestTypesallLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("typesall")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range ktypesallDecks {
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
	tbl, _, _, err := set.Matchup(ktypesallDecks[0], ktypesallDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	// instance index -> card name (Canon.Src is the deck instance)
	var inst []string
	for _, d := range decks {
		for _, c := range d {
			inst = append(inst, cardName(c))
		}
	}
	games, carrierPlays, ensouls, cycles := 0, 0, 0, 0
	for seed := uint64(1); seed <= 48; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"ktypesall-a", "ktypesall-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		// every other seed casts or activates whenever it can (the
		// anthems, Overseer taps and cycles land early); the rest are
		// random
		pol := castActivatePolicy(seed)
		if seed%2 == 1 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		games++
		for _, a := range res.Actions {
			k := a.Kind()
			if k != ActCast && k != ActActivate {
				continue
			}
			x := int(a.Src())
			if x >= len(inst) {
				continue
			}
			n := inst[x]
			if ktypesallCarriers[n] {
				carrierPlays++
			}
			if n == "Ensoul Artifact" {
				ensouls++
			}
			if n == "Sojourner's Companion" && k == ActActivate {
				cycles++
			}
		}
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d carrier plays, %d Ensoul Artifact casts, %d artifact landcycles", games, carrierPlays, ensouls, cycles)
	if carrierPlays < 48 || ensouls == 0 || cycles == 0 {
		t.Fatalf("the carriers were not exercised: %d carrier plays, %d ensouls, %d cycles", carrierPlays, ensouls, cycles)
	}
}
