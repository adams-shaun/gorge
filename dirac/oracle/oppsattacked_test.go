package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The opponents-attacked lockstep: the Battlefront cards read
// PlayerCountPropertyYou$OpponentsAttackedThisTurn (kernel ExOppsAttacked),
// as the amount an Attacks trigger draws, pumps or creates, as the bound of
// an "up to X targets" slot, and as the token count of a tapped-and-attacking
// token. Both decks hold Ajani, Caller of the Pride, so an attack aimed at a
// planeswalker (which names no player) is played beside one aimed at the player.

// koppsDecks holds two matchups. The first plays Rite of Ascension against
// a deck with no planeswalker: its tapped-and-attacking tokens ask the
// defender when the opponent has a planeswalker, which dirac declares a limit
// (StallTokAtkAsk), so the second matchup, where the opponent holds Ajani,
// Caller of the Pride and attacks may name the planeswalker, leaves Rite out.
var koppsDecks = [2][2][]cardset.Entry{
	{
		{
			{Name: "Korok, Liberated Rage", Count: 2}, {Name: "Kurn, Son of Mogh", Count: 2},
			{Name: "Martok, Hero of the Empire", Count: 2}, {Name: "Galvanizing Tenor", Count: 2},
			{Name: "Worf, Defiant Commander", Count: 1}, {Name: "Rite of Ascension", Count: 3},
			{Name: "Grizzly Bears", Count: 5}, {Name: "Hill Giant", Count: 3},
			{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 5},
		},
		{
			{Name: "Grizzly Bears", Count: 8}, {Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 6},
			{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
		},
	},
	{
		{
			{Name: "Korok, Liberated Rage", Count: 3}, {Name: "Kurn, Son of Mogh", Count: 3},
			{Name: "Martok, Hero of the Empire", Count: 3}, {Name: "Galvanizing Tenor", Count: 3},
			{Name: "Worf, Defiant Commander", Count: 1},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
			{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 7}, {Name: "Swamp", Count: 5},
		},
		{
			{Name: "Ajani, Caller of the Pride", Count: 3},
			{Name: "Grizzly Bears", Count: 6}, {Name: "Hill Giant", Count: 6}, {Name: "Raging Goblin", Count: 5},
			{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
		},
	},
}

var koppsCarriers = map[string]bool{
	"Korok, Liberated Rage": true, "Kurn, Son of Mogh": true, "Martok, Hero of the Empire": true,
	"Galvanizing Tenor": true, "Worf, Defiant Commander": true, "Rite of Ascension": true,
}

func TestOppsAttackedLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("oppsattacked")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][2][]*cards.Card
	var inst [2][]string
	var tbls [2]*kop.Table
	for v, md := range koppsDecks {
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for i := 0; i < e.Count; i++ {
					decks[v][s] = append(decks[v][s], c)
					inst[v] = append(inst[v], e.Name)
				}
			}
			if len(decks[v][s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", v, s, len(decks[v][s]))
			}
		}
		tbls[v], _, _, err = set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
	}
	attacks := 0
	for seed := uint64(1); seed <= 192; seed++ {
		v := int(seed % 2)
		cfg := Config{GorgeAutopaySurface: seed%4 >= 2}
		m, err := NewMatchup(reg, [2]string{"kopps-a", "kopps-b"}, decks[v], 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbls[v], cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		pol := klowlifePolicy(seed)
		if seed%3 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		for _, a := range res.Actions {
			if x := int(a.Src()); a.Kind() == ActAttack && x < len(inst[v]) && koppsCarriers[inst[v][x]] {
				attacks++
			}
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if s := res.Stall; s != "" && s != "turns" {
			t.Errorf("seed %d: stall %s", seed, s)
		}
	}
	t.Logf("%d carrier attacks over 192 games", attacks)
	if attacks < 40 {
		t.Errorf("only %d carrier attacks: the lockstep barely reaches the read", attacks)
	}
}
