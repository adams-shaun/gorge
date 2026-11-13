package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The kcycle lockstep: constructed matchups over the validaggs pool's hand
// abilities -- basic landcycling (Kulrath Zealot), forest/plains
// landcycling (Orchard Strider, Kree Sentinel), wizardcycling (Vedalken
// Aethermage), islandcycling (Shoreline Ranger), plain cycling
// (Agonasaur Rex) and channel (Ironhoof Boar, Bamboo Grove Archer) -- run
// WITHOUT pass elision, so gorge's singleton Discard<1/CARDNAME> ask (the
// oracle's selfDiscardCost) and the cyclers' battlefield keywords
// (projchars gorgeKwElsewhere) are both on the path. Matchup 1 is the
// ExSumPower carrier (kcycle item 3).

var kcycleDecks = [][2][]cardset.Entry{{
	{
		{Name: "Kulrath Zealot", Count: 4}, {Name: "Orchard Strider", Count: 4}, {Name: "Agonasaur Rex", Count: 3},
		{Name: "Ironhoof Boar", Count: 3}, {Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Mountain", Count: 9}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Kree Sentinel", Count: 4}, {Name: "Vedalken Aethermage", Count: 4}, {Name: "Shoreline Ranger", Count: 3},
		{Name: "Bamboo Grove Archer", Count: 3}, {Name: "Coral Merfolk", Count: 4}, {Name: "Giant Spider", Count: 4},
		{Name: "Island", Count: 9}, {Name: "Forest", Count: 9},
	},
}, {
	// Count$Valid ...$CardPower (ExSumPower) beside an anthem and pumps, so
	// derived and printed power differ: gorge sums current power on the
	// battlefield (zoneFoldPropCardPower, cr 4ca21a7dd); Ghalta's cost,
	// Volcanic Salvo's cost, pack tactics, formidable, Owlbear Shepherd
	{
		{Name: "Ghalta, Primal Hunger", Count: 2}, {Name: "Volcanic Salvo", Count: 2}, {Name: "Gnoll Hunter", Count: 3},
		{Name: "Hobgoblin Captain", Count: 3}, {Name: "Owlbear Shepherd", Count: 2}, {Name: "Dragon-Scarred Bear", Count: 2},
		{Name: "Crater Elemental", Count: 2}, {Name: "Giant Growth", Count: 3}, {Name: "Trumpet Blast", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Mountain", Count: 8}, {Name: "Forest", Count: 9},
	},
	{
		{Name: "Glorious Anthem", Count: 3}, {Name: "Savannah Lions", Count: 4}, {Name: "Gnoll Hunter", Count: 3},
		{Name: "Owlbear Shepherd", Count: 3}, {Name: "Giant Growth", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Forest", Count: 10},
	},
}}

func TestCycleLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("validaggs")
	if err != nil {
		t.Fatal(err)
	}
	games, acts, onField := 0, 0, 0
	for mi, md := range kcycleDecks {
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
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"kcycle-a", "kcycle-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			// every other seed casts or activates whenever it can (the
			// cycles); the rest are random, so cyclers also resolve onto
			// the battlefield and their keywords are projected
			pol := castActivatePolicy(seed)
			if seed%2 == 1 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			for _, a := range res.Actions {
				switch a.Kind() {
				case ActActivate:
					acts++
				case ActCast:
					onField++
				}
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
	t.Logf("%d games, %d activations, %d casts", games, acts, onField)
	if acts == 0 || onField == 0 {
		t.Fatal("no activation or cast was played")
	}
}
