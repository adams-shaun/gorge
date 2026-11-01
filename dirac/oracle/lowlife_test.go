package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lowest-life lockstep: the ten DSK "enters tapped
// unless a player has 13 or less life" lands (PlayerCountPlayers$
// LowestLifeTotal, kernel ExLowestLife), so the ETB-tap replacement's
// condition is decided at every land play on both sides of 13 life. The
// frozen FDN and BLB pair pools carry none of these lands.

var klowlifeDecks = [][2][]cardset.Entry{{
	// A (RG burn): three of the lands, burn to cross the 13 threshold,
	// vanilla creatures and basics
	{
		{Name: "Bleeding Woods", Count: 4}, {Name: "Razortrap Gorge", Count: 4}, {Name: "Raucous Carnival", Count: 4},
		{Name: "Lightning Bolt", Count: 4}, {Name: "Shock", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	// B (WUB): the other seven lands, vanilla creatures and basics
	{
		{Name: "Abandoned Campground", Count: 2}, {Name: "Etched Cornfield", Count: 2}, {Name: "Lakeside Shack", Count: 2},
		{Name: "Murky Sewer", Count: 2}, {Name: "Neglected Manor", Count: 2}, {Name: "Peculiar Lighthouse", Count: 2},
		{Name: "Strangled Cemetery", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Plains", Count: 7}, {Name: "Island", Count: 7}, {Name: "Swamp", Count: 4},
	},
}}

// klowlifeLands names the ten carriers whose battlefield entries the test
// classifies from gorge's event log.
func klowlifeLands(name string) bool {
	switch name {
	case "Abandoned Campground", "Bleeding Woods", "Etched Cornfield", "Lakeside Shack",
		"Murky Sewer", "Neglected Manor", "Peculiar Lighthouse", "Raucous Carnival",
		"Razortrap Gorge", "Strangled Cemetery":
		return true
	}
	return false
}

// klowlifeCounts is the ETB classification the check hook accumulates.
type klowlifeCounts struct {
	seen    int      // land entries classified so far
	tapHigh int      // entered tapped while every life total was > 13
	stayLow int      // entered untapped while some player was at <= 13
	odd     []uint64 // entries whose tap state disagreed with the 13 rule (seed)
}

// klowlifePolicy plays a land whenever it can, then casts a spell, then
// attacks, then aims spells at a player, and never blocks (so the games end
// inside the turn cap, with plenty of land entries on both sides of the 13
// threshold).
func klowlifePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for _, k := range []CanonKind{ActPlayLand, ActCast, ActAttack} {
				for _, a := range c.D.Options {
					if a.Kind() == k {
						return a
					}
				}
			}
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok {
					if _, isP := o.Obj.PlayerRef(); isP {
						return a
					}
				}
			}
		}
		for _, a := range c.D.Options {
			if a.Kind() == ActPlayLand {
				return a
			}
		}
		for _, a := range c.D.Options {
			switch a.Kind() {
			case ActCast, ActAttack, ActBlock, ActTarget:
				continue
			}
			return a
		}
		return rnd.Pick(c)
	})
}

// klowlifeScan is the classifier's state between check-hook calls: a land
// entry whose ETB tap has not been seen yet.
type klowlifeScan struct {
	entry   state.ObjID
	pending bool
}

// klowlifeWatch scans the gorge log since the last decision, classifying
// each of the ten lands' battlefield entries: the ETB tap is the event
// touching the entering object BEFORE the next priority boundary (the tap is
// part of the move's replacement, so it lands before the ask that follows
// the play); once a boundary comes first, or another land enters, or the
// game ends, the land entered untapped. The life total read at the tap is
// the one the replacement read: nothing between a land's entry and the next
// priority ask changes life.
func klowlifeWatch(g *GorgeSide, c *klowlifeCounts, sc *klowlifeScan, last *int, seed uint64) *Divergence {
	evs := g.E.L.Events
	life := func() int {
		l0, l1 := g.E.G.Players[0].Life, g.E.G.Players[1].Life
		if l1 < l0 {
			return int(l1)
		}
		return int(l0)
	}
	settle := func(tapped bool) {
		min := life()
		switch {
		case tapped && min > 13:
			c.tapHigh++
		case !tapped && min <= 13:
			c.stayLow++
		default:
			c.odd = append(c.odd, seed)
		}
		c.seen++
	}
	for i := *last; i < len(evs); i++ {
		e := &evs[i]
		if sc.pending {
			if e.Obj == sc.entry {
				settle(e.Kind == events.Tap)
				sc.pending = false
			} else {
				switch e.Kind {
				case events.Priority, events.DecisionAsk, events.DecisionMade, events.ManaActivate, events.StepChange, events.TurnChange:
					settle(false)
					sc.pending = false
				}
			}
		}
		if e.Kind == events.MoveZone && e.To == state.ZBattlefield {
			if sc.pending {
				settle(false)
			}
			if o := g.E.G.Obj(e.Obj); o != nil && o.Face() != nil && klowlifeLands(o.Face().Name) {
				sc.entry, sc.pending = e.Obj, true
			}
		}
	}
	// a land still pending at the end of the game entered untapped
	if sc.pending {
		settle(false)
		sc.pending = false
	}
	*last = len(evs)
	return nil
}

// runLowlife plays the constructed matchup and returns the games, the
// divergences and the ETB classification. mutate flips the reader to the
// highest life total (the mutation must diverge).
func runLowlife(t *testing.T, seeds int, mutate bool) (games, divs int, counts *klowlifeCounts) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("lowlife")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugLowLifeMax
	dirac.DebugLowLifeMax = mutate
	defer func() { dirac.DebugLowLifeMax = old }()
	counts = &klowlifeCounts{}
	md := klowlifeDecks[0]
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
			t.Fatalf("deck %d: %d cards", s, len(decks[s]))
		}
	}
	tbl, _, _, err := set.Matchup(md[0], md[1])
	if err != nil {
		t.Fatal(err)
	}
	last := 0
	var scan klowlifeScan
	curSeed := uint64(0)
	check := func(ref *GorgeSide, _ Side) *Divergence {
		return klowlifeWatch(ref, counts, &scan, &last, curSeed)
	}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"klowlife-a", "klowlife-b"}, decks, 1)
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
		pol := klowlifePolicy(seed)
		last = 0
		scan = klowlifeScan{}
		curSeed = seed
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 30, Check: check})
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		if s := res.Stall; s != "" {
			t.Errorf("seed %d: stall %s (turns %d, decisions %d, life %d/%d)", seed, s, res.Turns, res.Decisions, g.E.G.Players[0].Life, g.E.G.Players[1].Life)
		}
	}
	return games, divs, counts
}

func TestLowlifeLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, c := runLowlife(t, 256, false)
	t.Logf("%d games, %d divergences, %d land entries, %d tapped above 13, %d untapped at 13 or less, odd %d",
		games, divs, c.seen, c.tapHigh, c.stayLow, len(c.odd))
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	if len(c.odd) != 0 {
		t.Fatalf("gorge's ETB-tap condition disagreed with the min life reading on seeds %v (stop rule: report)", c.odd)
	}
	if c.tapHigh < 20 {
		t.Errorf("only %d lands entered tapped above 13 life", c.tapHigh)
	}
	if c.stayLow < 20 {
		t.Errorf("only %d lands entered untapped at 13 or less life", c.stayLow)
	}
}

// TestLowlifeMutation flips the reader to the highest life total: the
// cards read the LOWEST, so the games must diverge. A green mutation (no
// divergence) means the lockstep never reaches a 13-threshold decision.
func TestLowlifeMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _ := runLowlife(t, 16, true)
	if divs == 0 {
		t.Fatalf("mutation read the highest life total in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
