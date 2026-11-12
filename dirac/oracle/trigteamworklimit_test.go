package oracle

import (
	"math/rand/v2"
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The trigger-parameter lockstep: Teamwork$ True on a
// Taps line (Agent Maria Hill), GameActivationLimit$ 1 (Acrobatic
// Cheerleader) and an inert TriggerController$ TriggeredCardController on a
// Discarded line (Titanbones, Towering Heart). Two decks, played with the
// seats both ways; gorge leads (LockstepLed). The policy attacks whenever it
// can, elects teamwork whenever it is offered and plays at random otherwise.
//
// Tallies come from gorge's event log: Maria's taps split into teamwork
// taps (Counter events.TapTeamworkCounter) and her other taps (attacks),
// her Taps triggers; Cheerleader's triggers and the stints where a second
// tapped main phase of its controller did not trigger it again; Titanbones'
// triggers pushed from the graveyard (the discard line).

var ktrigp3Decks = [2][]cardset.Entry{
	{ // A: Maria, Team Tactics and Burning-Tree Emissary (the floating mana that gets a teamworked cast offered) in numbers
		{Name: "Agent Maria Hill", Count: 6}, {Name: "Acrobatic Cheerleader", Count: 4},
		{Name: "Helicarrier Strike", Count: 2}, {Name: "Team Tactics", Count: 6},
		{Name: "Burning-Tree Emissary", Count: 6},
		{Name: "Plains", Count: 8}, {Name: "Mountain", Count: 8},
	},
	{ // B
		{Name: "Titanbones, Towering Heart", Count: 4}, {Name: "Tormenting Voice", Count: 4},
		{Name: "Thrill of Possibility", Count: 4}, {Name: "Faithless Looting", Count: 2},
		{Name: "Grizzly Bears", Count: 6}, {Name: "Llanowar Elves", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 8},
	},
}

type ktrigp3Tally struct {
	evAt                         int
	twTaps, plainTaps, mariaTrig int
	cheerTrig, cheerSupp         int
	titanTrig                    int
	moves                        map[state.ObjID]int
	zone                         map[state.ObjID]state.Zone
	cheer                        map[state.ObjID]bool
	m2seen                       map[[2]int32]bool
	stint                        map[[2]int]*[2]int // (obj, moves) -> tapped main-2s, pushes
}

func (c *ktrigp3Tally) reset() {
	c.evAt = 0
	c.moves = map[state.ObjID]int{}
	c.zone = map[state.ObjID]state.Zone{}
	c.cheer = map[state.ObjID]bool{}
	c.m2seen = map[[2]int32]bool{}
	c.stint = map[[2]int]*[2]int{}
}

func (c *ktrigp3Tally) st(o state.ObjID) *[2]int {
	k := [2]int{int(o), c.moves[o]}
	if c.stint[k] == nil {
		c.stint[k] = new([2]int)
	}
	return c.stint[k]
}

// finish folds the per-stint records of one game into the tallies.
func (c *ktrigp3Tally) finish() {
	for _, r := range c.stint {
		if r[0] >= 2 && r[1] == 1 {
			c.cheerSupp++
		}
	}
}

func (c *ktrigp3Tally) add(o *ktrigp3Tally) {
	c.twTaps += o.twTaps
	c.plainTaps += o.plainTaps
	c.mariaTrig += o.mariaTrig
	c.cheerTrig += o.cheerTrig
	c.cheerSupp += o.cheerSupp
	c.titanTrig += o.titanTrig
}

func (c *ktrigp3Tally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < c.evAt {
		c.evAt = 0
	}
	for j := c.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.MoveZone:
			c.moves[e.Obj]++
			c.zone[e.Obj] = e.To
			if cpName(g, e.Obj) == "Acrobatic Cheerleader" {
				c.cheer[e.Obj] = true
			}
		case events.Tap:
			if cpName(g, e.Obj) == "Agent Maria Hill" {
				if e.Counter == events.TapTeamworkCounter {
					c.twTaps++
				} else {
					c.plainTaps++
				}
			}
		case events.TriggerPush:
			switch cpName(g, e.Obj) {
			case "Agent Maria Hill":
				c.mariaTrig++
			case "Acrobatic Cheerleader":
				c.cheerTrig++
				c.st(e.Obj)[1]++
			case "Titanbones, Towering Heart":
				if c.zone[e.Obj] == state.ZGraveyard {
					c.titanTrig++
				}
			}
		}
	}
	c.evAt = len(evs)
	if g.Step == state.StepMain2 {
		for id := range c.cheer {
			o := g.Obj(id)
			k := [2]int32{int32(id), g.Turn}
			if o == nil || o.Zone != state.ZBattlefield || !o.Tapped || o.Controller != g.Active || c.m2seen[k] {
				continue
			}
			c.m2seen[k] = true
			c.st(id)[0]++
		}
	}
	return nil
}

// ktrigp3Policy attacks whenever it can, elects teamwork whenever it is
// offered (a creature pick while one is offered, a teamworked cast beside the
// plain one) and plays at random otherwise.
func ktrigp3Policy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	prng := rand.New(rand.NewPCG(seed, 0x7472696770333)) // "trigp3"
	return PolicyFunc(func(p *PickCtx) Canon {
		switch p.D.Kind {
		case DChoose:
			if code, ok := chooseCode("teamwork"); ok {
				var picks []Canon
				for _, a := range p.D.Options {
					if a.Kind() == ActChoose+CanonKind(code) {
						picks = append(picks, a)
					}
				}
				if len(picks) > 0 {
					for _, a := range picks {
						if refCardName(p.G, a.Src()) == "Agent Maria Hill" {
							return a // the tap that pays for Maria's trigger
						}
					}
					return picks[prng.IntN(len(picks))]
				}
			}
		case DAttackers:
			var atk []Canon
			for _, a := range p.D.Options {
				if a.Kind() == ActAttack {
					atk = append(atk, a)
				}
			}
			if len(atk) > 0 {
				return atk[prng.IntN(len(atk))]
			}
		case DPriority:
			for _, a := range p.D.Options {
				if a.Kind() == ActCast && a.Mode()>>4 == 0 && a.Mode()&0xf == castModeOther {
					return a
				}
			}
			// gorge's planner prices no teamworked cast: only the mana a
			// Burning-Tree Emissary floats as it enters gets it offered. Maria
			// first, so she is there to be tapped.
			for _, n := range []string{"Agent Maria Hill", "Burning-Tree Emissary"} {
				for _, a := range p.D.Options {
					if a.Kind() == ActCast && refCardName(p.G, a.Src()) == n {
						return a
					}
				}
			}
		}
		return rnd.Pick(p)
	})
}

// ktrigp3Sweep plays seeds 1..n over four workers, the seats both ways, and
// returns the merged tally, the number of divergences and stalls.
func ktrigp3Sweep(t *testing.T, n uint64) (*ktrigp3Tally, int, int) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("trigteamworklimit")
	if err != nil {
		t.Fatal(err)
	}
	var cs [2][]*cards.Card // by deck
	for d, es := range ktrigp3Decks {
		for _, e := range es {
			card, ok := reg.Lookup(e.Name)
			if !ok {
				t.Fatalf("deck %d: %s not in the corpus", d, e.Name)
			}
			for i := 0; i < e.Count; i++ {
				cs[d] = append(cs[d], card)
			}
		}
		if len(cs[d]) != 40 {
			t.Fatalf("deck %d: %d cards", d, len(cs[d]))
		}
	}
	const workers = 4
	var (
		wg           sync.WaitGroup
		mu           sync.Mutex
		total        = &ktrigp3Tally{}
		divs, stalls int
	)
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			c := &ktrigp3Tally{}
			var wd, ws int
			for seed := uint64(w + 1); seed <= n; seed += workers {
				swap := seed&2 != 0 // the seats both ways
				names := [2]string{"trigp3-a", "trigp3-b"}
				ents := ktrigp3Decks
				decks := cs
				if swap {
					names[0], names[1] = names[1], names[0]
					ents[0], ents[1] = ents[1], ents[0]
					decks[0], decks[1] = decks[1], decks[0]
				}
				tbl, _, _, err := set.Matchup(ents[0], ents[1])
				if err != nil {
					t.Errorf("seed %d: matchup: %v", seed, err)
					return
				}
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, names, decks, 1)
				if err != nil {
					t.Errorf("seed %d: matchup: %v", seed, err)
					return
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Errorf("seed %d: side: %v", seed, err)
					return
				}
				lead, err := NewGorgeSide(m, seed, cfg, nil)
				if err != nil {
					t.Errorf("seed %d: gorge: %v", seed, err)
					return
				}
				c.reset()
				res := LockstepLed(m, seed, lead, k, ktrigp3Policy(seed),
					RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: c.check})
				CloseSide(k)
				c.finish()
				if d := res.Div; d != nil {
					wd++
					if t.Name() == "TestTeamworkActivationLimitLockstep" {
						lo := max(d.At-12, 0)
						t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
						t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
							seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					}
				}
				if res.Stall != "" && res.Stall != "turns" && res.Stall != "decisions" {
					ws++
					if t.Name() == "TestTeamworkActivationLimitLockstep" {
						t.Errorf("seed %d: stalled: %s after %d turns", seed, res.Stall, res.Turns)
					}
				}
			}
			mu.Lock()
			total.add(c)
			divs += wd
			stalls += ws
			mu.Unlock()
		}(w)
	}
	wg.Wait()
	return total, divs, stalls
}

func TestTeamworkActivationLimitLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	c, divs, stalls := ktrigp3Sweep(t, 256)
	t.Logf("256 games: %d divergences, %d stalls; Maria teamwork taps %d, other taps %d, triggers %d; Cheerleader triggers %d, suppressed second main-2 stints %d; Titanbones discard triggers %d",
		divs, stalls, c.twTaps, c.plainTaps, c.mariaTrig, c.cheerTrig, c.cheerSupp, c.titanTrig)
	if c.mariaTrig != c.twTaps {
		t.Errorf("Maria triggers %d != teamwork taps %d (a plain tap triggered, or a teamwork tap did not)", c.mariaTrig, c.twTaps)
	}
	for _, a := range []struct {
		name string
		got  int
	}{
		{"Maria teamwork taps", c.twTaps},
		{"Maria other taps (attacks)", c.plainTaps},
		{"Cheerleader triggers", c.cheerTrig},
		{"Cheerleader second tapped main-2 not triggering", c.cheerSupp},
		{"Titanbones discard triggers", c.titanTrig},
	} {
		if a.got < 3 {
			t.Errorf("%s: %d, want >= 3", a.name, a.got)
		}
	}
}

// TestTeamworkActivationLimitMutation: each hook must make the kernel diverge from gorge.
func TestTeamworkActivationLimitMutation(t *testing.T) {
	for _, m := range []struct {
		name string
		hook *bool
	}{
		{"TWTapAny", &dirac.DebugTWTapAny},
		{"GameOnceOff", &dirac.DebugGameOnceOff},
	} {
		t.Run(m.name, func(t *testing.T) {
			*m.hook = true
			defer func() { *m.hook = false }()
			_, divs, _ := ktrigp3Sweep(t, 64)
			t.Logf("%s: %d of 64 games diverged", m.name, divs)
			if divs == 0 {
				t.Errorf("%s: no divergence in 64 seeds", m.name)
			}
		})
	}
}
