package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// Lion's Eye Diamond ("Discard your hand, Sacrifice this: Add three mana of
// any one color. Activate only as an instant.") is a mana ability gorge keeps
// out of every payment window and its planner census (CR 605.4: activated
// only at instant timing); only a potential play's prefix search activates
// it. Jayemdae Tome's {4}, {T} ability is a play from the battlefield the
// discarded hand does not take with it, so beside one Mountain-paid land the
// Diamond's three mana funds it; every hand cast the Diamond would fund is
// discarded by its own cost. The matchups run the Tome player against plain
// creatures, gorge's two surfaces alternating by seed.
var ledDecks = [2][]cardset.Entry{
	{
		{Name: "Lion's Eye Diamond", Count: 4}, {Name: "Jayemdae Tome", Count: 4},
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
	{
		{Name: "Hill Giant", Count: 6}, {Name: "Grizzly Bears", Count: 8}, {Name: "Gray Ogre", Count: 6},
		{Name: "Mountain", Count: 10}, {Name: "Forest", Count: 10},
	},
}

// ledPolicy activates whenever it can (the Tome's draw, the Diamond-scripted
// one among them) and otherwise plays at random.
func ledPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		for _, a := range c.D.Options {
			if a.Kind() == ActActivate {
				return a
			}
		}
		return rnd.Pick(c)
	})
}

// TestLionsEyeDiamondLockstep: gorge and dirac play the Diamond's scripted
// activation identically, and the games do activate it (a Diamond reaches a
// graveyard while its controller's hand empties into it).
func TestLionsEyeDiamondLockstep(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	var names [2][]string
	var decks [2][]*cards.Card
	for s, d := range ledDecks {
		for _, e := range d {
			c, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("%s not in the corpus", e.Name)
			}
			names[s] = append(names[s], e.Name)
			for i := 0; i < e.Count; i++ {
				decks[s] = append(decks[s], c)
			}
		}
		if len(decks[s]) != 40 {
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	set := printedDelveSet(t, names)
	tbl, _, _, err := set.Matchup(ledDecks[0], ledDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	sacked := map[state.ObjID]bool{}
	discarded := 0
	games := 0
	for seed := uint64(1); seed <= 48; seed++ {
		m, err := NewMatchup(reg, [2]string{"led-a", "led-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[state.ObjID]bool{} // a Diamond sacrificed from the battlefield
		onBF := map[state.ObjID]bool{}
		hand := 0
		check := func(ref *GorgeSide, _ Side) *Divergence {
			g := ref.E.G
			for _, id := range g.Zone(state.ZGraveyard, 0) {
				if onBF[id] && !seen[id] {
					seen[id] = true
					if hand > 0 {
						discarded++ // the activation discarded a non-empty hand
					}
				}
			}
			for _, id := range g.Zone(state.ZBattlefield, 0) {
				if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Lion's Eye Diamond" {
					onBF[id] = true
				}
			}
			hand = len(g.Zone(state.ZHand, 0))
			return nil
		}
		res := Lockstep(m, seed, k, ledPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 1500, MaxTurns: 16, Check: check})
		CloseSide(k)
		games++
		for id := range seen {
			sacked[ledObjKey(seed, id)] = true
		}
		if d := res.Div; d != nil {
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games, %d Diamonds sacrificed, %d of them with cards in hand", games, len(sacked), discarded)
	if len(sacked) < 5 || discarded < 3 {
		t.Errorf("Diamonds sacrificed: %d (%d discarding a hand), want at least 5 (3): the scripted activation is not exercised", len(sacked), discarded)
	}
}

// ledObjKey keys an object of one seed's game.
func ledObjKey(seed uint64, id state.ObjID) state.ObjID { return state.ObjID(seed<<20) | id }
