package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// Ertha Jo, Frontier Mentor lowers: "whenever you activate an ability that
// targets a creature or player, copy that ability. You may choose new targets
// for the copy" (Mode$ AbilityCast, ValidSA$ SpellAbility, TargetsValid$
// Player,Creature.inZoneBattlefield, then DB$ CopySpellAbility, Defined$
// TriggeredSpellAbility). Her Mercenary token, Prodigal Sorcerer and Mogg
// Fanatic all activate targeted abilities (a creature or a player); Llanowar
// Elves's mana ability has no target, so it must not fire her. The lockstep
// compares gorge's copies and the new-target asks with the kernel's.

var kErthaJoDecks = [2][]cardset.Entry{
	{
		{Name: "Ertha Jo, Frontier Mentor", Count: 4}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Mogg Fanatic", Count: 4},
		{Name: "Llanowar Elves", Count: 3}, {Name: "Grizzly Bears", Count: 3}, {Name: "Shock", Count: 2},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 7}, {Name: "Plains", Count: 7},
	},
	{
		{Name: "Ertha Jo, Frontier Mentor", Count: 2}, {Name: "Prodigal Sorcerer", Count: 4}, {Name: "Mogg Fanatic", Count: 3},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Forest", Count: 6}, {Name: "Mountain", Count: 7}, {Name: "Island", Count: 7},
	},
}

func TestErthaJoLowers(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("erthajo")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kErthaJoDecks {
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
	// Ertha Jo lowers: Matchup refuses a card the pool's table lacks
	tbl, _, _, err := set.Matchup(kErthaJoDecks[0], kErthaJoDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	copies, games := 0, 0
	for seed := uint64(1); seed <= 120; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"erthajo-a", "erthajo-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, castActivatePlayedPolicy(seed, map[string]int{}), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			t.Fatalf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
		for i := range g.E.G.Objs {
			o := &g.E.G.Objs[i]
			if o.Ability == nil || o.Ability.API != "CopySpellAbility" {
				continue
			}
			if src := g.E.G.Obj(o.Source); src != nil && src.Card != nil && src.Card.Faces[0].Name == "Ertha Jo, Frontier Mentor" {
				copies++
			}
		}
	}
	t.Logf("%d games; Ertha Jo copy triggers %d", games, copies)
	if copies < 10 {
		t.Errorf("only %d Ertha Jo copy triggers reached the stack (need 10)", copies)
	}
}
