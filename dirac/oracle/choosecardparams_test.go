package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
)

// The mandatory-ChooseCard default-params lockstep:
// Riverwheel Sweep, Heroes' Hangout and Mishra's Research Desk read Amount$ 1,
// Defined$ You and ChoiceTitle$ on a mandatory exile-zone Card.IsRemembered
// pick. Before the fix the shared unread check refused all three
// (param:ChooseCard.ChoiceTitle, .Amount, .Defined). Riverwheel is in the pool
// but gorge never offers it as a cast (reported as a gorge finding); the games
// pose Heroes' Hangout and Mishra's Research Desk, and Lord of the Void is the
// same mandatory pick on a combat-damage trigger whose 7/7 flying body also
// closes the 20-turn games. Gorge leads and asks the resolving controller for
// one card; the kernel must agree on every option and every projection.
var kccpDecks = [2][]cardset.Entry{
	{
		// the carrier seat: the two default-params ChooseCards the games
		// pose, beside a vanilla board
		{Name: "Heroes' Hangout", Count: 4}, {Name: "Mishra's Research Desk", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Llanowar Elves", Count: 4},
		{Name: "Captivating Cave", Count: 4}, {Name: "Forest", Count: 4},
		{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 4},
	},
	{
		// the beating seat: Lord of the Void needs the game to last long
		// enough to connect, so the deck carries six and only two Craw
		// Wurms; the carrier seat's ground board cannot race it
		{Name: "Lord of the Void", Count: 6}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Craw Wurm", Count: 2},
		{Name: "Llanowar Elves", Count: 4},
		{Name: "Captivating Cave", Count: 4}, {Name: "Forest", Count: 5},
		{Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4}, {Name: "Plains", Count: 3},
	},
}

var kccpCarriers = map[string]bool{
	"Riverwheel Sweep":       true,
	"Heroes' Hangout":        true,
	"Mishra's Research Desk": true,
	"Crackling Doom":         true,
	"Lord of the Void":       true,
}

// kccpTally counts resolved ChooseCard asks from gorge's events: choiceRecord
// emits events.Choose with Counter "chosen" once per answered pick, keyed by
// the source card.
type kccpTally struct {
	evAt      int
	byCarrier map[string]int
}

func (p *kccpTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind != events.Choose || e.Counter != "chosen" {
			continue
		}
		if n := chName(g, e.Obj); kccpCarriers[n] {
			p.byCarrier[n]++
		}
	}
	p.evAt = len(evs)
	return nil
}

// kccpCreatureCast reports whether a casts a creature, the board development
// that lets the 20-turn games finish.
func kccpCreatureCast(c *PickCtx, a Canon) bool {
	if c.G == nil || a.Kind() != ActCast {
		return false
	}
	o, ok := c.G.GorgeOption(a)
	if !ok {
		return false
	}
	ob := c.G.E.G.Obj(o.Obj)
	return ob != nil && ob.Face() != nil && ob.Face().IsCreature()
}

// kccpPolicy plays a land, then a creature, prefers Mishra's activated ability
// so its ChooseCard is reached, and otherwise casts; a mode ask takes the
// first offered mode (Date Night, the ChooseCard mode of Heroes' Hangout).
// Everything else random.
func kccpPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		switch c.D.Kind {
		case DPriority:
			for _, a := range c.D.Options {
				if a.Kind() == ActPlayLand {
					return a
				}
			}
			for _, a := range c.D.Options {
				if kccpCreatureCast(c, a) {
					return a
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() != ActActivate || c.G == nil {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok && chName(c.G.E.G, o.Obj) == "Mishra's Research Desk" {
					return a
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() == ActCast {
					return a
				}
			}
		case DAttackers:
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					return a
				}
			}
		case DBlockers:
			// let the attackers through so the 20-turn games decide
			for _, a := range c.D.Options {
				if a.Kind() == ActDone {
					return a
				}
			}
		case DModes:
			for _, a := range c.D.Options {
				if a.Kind() == ActMode {
					return a
				}
			}
		}
		return rnd.Pick(c)
	})
}

func TestChoosecardparamsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("choosecardparams")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kccpDecks {
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
	tbl, _, _, err := set.Matchup(kccpDecks[0], kccpDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tally := &kccpTally{byCarrier: map[string]int{}}
	games, divs, stalls := 0, 0, 0
	for seed := uint64(1); seed <= 128; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kccp-a", "kccp-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.evAt = 0
		res := Lockstep(m, seed, k, kccpPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 6000, MaxTurns: 20, Check: tally.check})
		CloseSide(k)
		games++
		if res.Stall != "" {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	total, carriers := 0, 0
	for _, n := range []string{"Riverwheel Sweep", "Heroes' Hangout", "Mishra's Research Desk", "Crackling Doom", "Lord of the Void"} {
		if tally.byCarrier[n] > 0 {
			carriers++
		}
		total += tally.byCarrier[n]
	}
	t.Logf("%d games, %d divergences, %d stalls; %d resolved ChooseCard asks over %d carriers: %v",
		games, divs, stalls, total, carriers, tally.byCarrier)
	if total < 20 {
		t.Errorf("%d resolved ChooseCard asks, want at least 20", total)
	}
	if carriers < 3 {
		t.Errorf("%d carriers asked, want at least 3: %v", carriers, tally.byCarrier)
	}
}
