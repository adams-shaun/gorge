package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The spawner-target lockstep: Biting-Palm Ninja's combat-damage trigger
// spawns an ImmediateTrigger whose body reveals the damaged player's hand
// and exiles a nonland card from it (Defined$ and DefinedPlayer$
// Spawner>TriggeredTarget, the one opponent in two players). Without the
// lowering the card is refused and the pool has no table for it.

var spawnerTargetDecks = [2][]cardset.Entry{
	{
		{Name: "Biting-Palm Ninja", Count: 6},
		{Name: "Hill Giant", Count: 5}, {Name: "Raging Goblin", Count: 6},
		{Name: "Swamp", Count: 12}, {Name: "Mountain", Count: 11},
	},
	{
		{Name: "Hill Giant", Count: 14}, {Name: "Raging Goblin", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Plains", Count: 10},
	},
}

func TestSpawnerTargetLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("spawnertarget")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	var inst []string
	for s, d := range spawnerTargetDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
				if s == 0 {
					inst = append(inst, e.Name)
				}
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	var tbl *kop.Table
	if tbl, _, _, err = set.Matchup(spawnerTargetDecks[0], spawnerTargetDecks[1]); err != nil {
		t.Fatal(err)
	}
	ninjaAttacks := 0
	for seed := uint64(1); seed <= 192; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 >= 2}
		m, err := NewMatchup(reg, [2]string{"spawnertarget-a", "spawnertarget-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
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
			if x := int(a.Src()); a.Kind() == ActAttack && x < len(inst) && inst[x] == "Biting-Palm Ninja" {
				ninjaAttacks++
			}
		}
		if d := res.Div; d != nil {
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		if s := res.Stall; s != "" && s != "turns" {
			t.Errorf("seed %d: stall %s", seed, s)
		}
	}
	t.Logf("%d Biting-Palm Ninja attacks over 192 games", ninjaAttacks)
	if ninjaAttacks < 20 {
		t.Errorf("only %d Biting-Palm Ninja attacks: the lockstep barely reaches the trigger", ninjaAttacks)
	}
}
