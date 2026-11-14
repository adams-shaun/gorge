package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The MBC trigger-referents-3 spelling family. Only Defined$
// TriggeredCardOwner lowers: the card is read off the trigger object's owner
// (kop.RefTriggerObjOwner; CR 108.3, gorge definedSpecTriggeredCardOwner).
// The other three spellings (TriggeredPlayersAttackedTarget$Amount, AbilityCast
// ValidSA$ SpellAbility, trigger mode Unattached) stay refused with their
// reasons; see the report.
//
// Liability ("whenever a nontoken permanent is put into a player's graveyard
// from the battlefield, that player loses 1 life") and Warped Devotion
// ("whenever a permanent is returned to a player's hand, that player discards
// a card") both address the owner of the permanent that moved. Each deck casts
// removal and bounce at the other side's permanents, so the permanent's owner
// is not the trigger's controller and the lockstep compares who loses the
// life or discards.

var kTrigOwnerDecks = [2][]cardset.Entry{
	{
		{Name: "Liability", Count: 4}, {Name: "Warped Devotion", Count: 4}, {Name: "Shock", Count: 3},
		{Name: "Doom Blade", Count: 3}, {Name: "Unsummon", Count: 3}, {Name: "Grizzly Bears", Count: 3},
		{Name: "Island", Count: 7}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Liability", Count: 2}, {Name: "Warped Devotion", Count: 2}, {Name: "Shock", Count: 3},
		{Name: "Doom Blade", Count: 3}, {Name: "Unsummon", Count: 3}, {Name: "Coral Merfolk", Count: 4},
		{Name: "Wind Drake", Count: 4}, {Name: "Island", Count: 9}, {Name: "Swamp", Count: 9}, {Name: "Plains", Count: 1},
	},
}

func TestMBCTriggerReferents3Lowers(t *testing.T) {
	t.Parallel()
	reg, err := cards.SharedCorpus(corpusDirOrSkip(t))
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigowner")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kTrigOwnerDecks {
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
	// the cards lower: Matchup refuses a card the pool's table lacks
	tbl, _, _, err := set.Matchup(kTrigOwnerDecks[0], kTrigOwnerDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	liability, devotion, games := 0, 0, 0
	for seed := uint64(1); seed <= 120; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"trigowner-a", "trigowner-b"}, decks, 1)
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
			if o.Ability == nil {
				continue
			}
			switch src := g.E.G.Obj(o.Source); {
			case src == nil || src.Card == nil:
			case o.Ability.API == "LoseLife" && src.Card.Faces[0].Name == "Liability":
				liability++
			case o.Ability.API == "Discard" && src.Card.Faces[0].Name == "Warped Devotion":
				devotion++
			}
		}
	}
	t.Logf("%d games; Liability triggers %d, Warped Devotion triggers %d", games, liability, devotion)
	if liability < 10 || devotion < 5 {
		t.Errorf("only %d Liability and %d Warped Devotion triggers reached the stack (need 10 and 5)", liability, devotion)
	}
}
