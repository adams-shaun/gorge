package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The lockstep: constructed matchups over the generated effectstatics pool -- the
// until-end-of-turn Effect statics (Goblin Diplomats' and Bident of Thassa's
// MustAttack, Dread Charge's CantBlockBy) and the later classes' carriers
// -- so the expected answer at every decision is gorge's. The frozen FDN and
// BLB pair pools exercise none of these arms.

var effectstaticsDecks = [][2][]cardset.Entry{{
	{
		{Name: "Goblin Diplomats", Count: 4}, {Name: "Hill Giant", Count: 3}, {Name: "Gray Ogre", Count: 3},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 2}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 12},
	},
	{
		{Name: "Dread Charge", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Vampire Cutthroat", Count: 4},
		{Name: "Bident of Thassa", Count: 3}, {Name: "Gray Ogre", Count: 1}, {Name: "Swamp", Count: 13}, {Name: "Island", Count: 11},
	},
}, {
	{
		{Name: "Reminisce", Count: 4}, {Name: "Feldon's Cane", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Gray Ogre", Count: 3}, {Name: "Giant Spider", Count: 2}, {Name: "Island", Count: 10}, {Name: "Forest", Count: 11},
	},
	{
		{Name: "Walking Corpse", Count: 4}, {Name: "Vampire Cutthroat", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Grizzly Bears", Count: 3}, {Name: "Swamp", Count: 11}, {Name: "Forest", Count: 10},
	},
}, {
	{
		{Name: "Expedition Raptor", Count: 4}, {Name: "Shoulder to Shoulder", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Walking Corpse", Count: 2}, {Name: "Plains", Count: 11}, {Name: "Swamp", Count: 6}, {Name: "Forest", Count: 5},
	},
	{
		{Name: "Goblin Diplomats", Count: 3}, {Name: "Hill Giant", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 2}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 11},
	},
}, {
	{
		{Name: "Irreverent Gremlin", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Giant Spider", Count: 2}, {Name: "Hill Giant", Count: 3}, {Name: "Mountain", Count: 12}, {Name: "Forest", Count: 11},
	},
	{
		{Name: "Nykthos Paragon", Count: 4}, {Name: "Vampire Cutthroat", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Gray Ogre", Count: 2}, {Name: "Shoulder to Shoulder", Count: 2}, {Name: "Plains", Count: 12}, {Name: "Swamp", Count: 10}, {Name: "Mountain", Count: 2},
	},
}}

// effectstaticsCarriers names the cards whose casts and activations the test counts
// (each must be played at least once).
var effectstaticsCarriers = []string{"Goblin Diplomats", "Bident of Thassa", "Dread Charge", "Reminisce", "Feldon's Cane", "Expedition Raptor", "Shoulder to Shoulder", "counter_pick", "Irreverent Gremlin", "Nykthos Paragon"}

// eagerPolicy casts or activates whenever it can (so the games reach the
// carriers), else random; it counts the carrier plays the gorge side poses.
func eagerPolicy(seed uint64, played map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if cp, _ := chooseCode("counter_pick"); a.Kind() == ActChoose+CanonKind(cp) {
			played["counter_pick"]++ // a support pick (the pools have no bolster)
		}
		if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

func TestEffectStaticsUntilEndOfTurnLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effectstatics")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	for mi, md := range effectstaticsDecks {
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
			m, err := NewMatchup(reg, [2]string{"c7b-a", "c7b-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := eagerPolicy(seed, played)
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
	for _, n := range effectstaticsCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
}
