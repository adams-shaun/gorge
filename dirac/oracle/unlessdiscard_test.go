package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
)

// The lockstep: constructed matchups over the unless-cost discard
// selection methods gorge made payable in d66db41d8 -- Discard<N/Random>
// (Balduvian Horde, Minotaur Explorer, Pillaging Horde, Gallia of the Endless
// Dance) and Discard<N/Hand> (Perplex) -- so every payment step (the engine
// RNG's picks, the whole-hand discard, the empty-hand pay) is compared against
// gorge's answer at every decision.

var unlessDiscardDecks = [][2][]cardset.Entry{{
	{
		{Name: "Balduvian Horde", Count: 3}, {Name: "Pillaging Horde", Count: 3}, {Name: "Minotaur Explorer", Count: 3},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 12},
	},
	{
		{Name: "Perplex", Count: 4}, {Name: "Divination", Count: 3}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Giant Spider", Count: 3}, {Name: "Centaur Courser", Count: 4},
		{Name: "Grizzly Bears", Count: 3}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 8},
	},
}, {
	{
		{Name: "Gallia of the Endless Dance", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Goblin Piker", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Centaur Courser", Count: 3},
		{Name: "Giant Spider", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3},
		{Name: "Mountain", Count: 6}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Pillaging Horde", Count: 4}, {Name: "Balduvian Horde", Count: 3}, {Name: "Minotaur Explorer", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 5},
	},
}, {
	{
		{Name: "Perplex", Count: 4}, {Name: "Divination", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Centaur Courser", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Island", Count: 6}, {Name: "Swamp", Count: 6},
	},
	{
		{Name: "Gallia of the Endless Dance", Count: 4}, {Name: "Raging Goblin", Count: 4},
		{Name: "Goblin Piker", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Centaur Courser", Count: 4},
		{Name: "Giant Spider", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}, {
	{
		{Name: "Pillaging Horde", Count: 4}, {Name: "Balduvian Horde", Count: 3}, {Name: "Minotaur Explorer", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Lightning Bolt", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 5},
	},
	{
		{Name: "Balduvian Horde", Count: 3}, {Name: "Pillaging Horde", Count: 3}, {Name: "Minotaur Explorer", Count: 3},
		{Name: "Raging Goblin", Count: 4}, {Name: "Goblin Piker", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 3}, {Name: "Shock", Count: 4}, {Name: "Mountain", Count: 12},
	},
}}

// unlessDiscardCarriers names the cards whose casts the test counts (each must be
// played at least once, so the unless-pay ask is reached).
var unlessDiscardCarriers = []string{"Balduvian Horde", "Minotaur Explorer", "Pillaging Horde", "Gallia of the Endless Dance", "Perplex"}

// unlessDiscardPolicy casts whenever it can (so the games reach the carriers),
// else random; it counts the carrier casts the gorge side poses.
func unlessDiscardPolicy(seed uint64, played map[string]int, payAsks map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		// The unless-pay ask is gorge's two-option KModes [pay, decline]
		// (kernel ChGenPayMode): take the pay branch so the discard payment
		// itself is exercised, not just the decline. Record the ask against
		// its source card, so each carrier's own payment is proved rather
		// than merely that some carrier paid.
		if c.G != nil {
			if d := c.G.Pending(); d != nil && d.Kind == decision.KModes && d.ResumeKind == decision.ModeUnlessPay &&
				c.D.Kind == DModes && len(c.D.Options) == 2 &&
				c.D.Options[0].Kind() == ActMode && c.D.Options[1].Kind() == ActMode {
				name := "?"
				if ob := c.G.E.G.Obj(d.Source); ob != nil && ob.Face() != nil {
					name = ob.Face().Name
				}
				payAsks[name]++
				return c.D.Options[0]
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
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
		if a.Kind() == ActCast && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					played[ob.Face().Name]++
				}
			}
		}
		return a
	})
}

func TestUnlessDiscardCostLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("unlessdiscard")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	played := map[string]int{}
	payAsks := map[string]int{}
	for mi, md := range unlessDiscardDecks {
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
			m, err := NewMatchup(reg, [2]string{"deck-a", "deck-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := unlessDiscardPolicy(seed, played, payAsks)
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
	t.Logf("%d games, carrier casts %v, unless-pay asks %v", games, played, payAsks)
	if payAsks["?"] != 0 {
		t.Errorf("%d unless-pay asks had no source card name", payAsks["?"])
	}
	for _, n := range unlessDiscardCarriers {
		if played[n] == 0 {
			t.Errorf("carrier %s never cast", n)
		}
		if payAsks[n] == 0 {
			t.Errorf("carrier %s never posed an unless-pay ask", n)
		}
	}
}
