package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The targeted/triggered-saddle lockstep: the three
// AlterAttribute Attributes$ Saddled carriers beside Mounts whose attack
// trigger reads IsSaddled and a BecomesSaddled carrier, so the op resolves a
// target (Guidelight Matrix, Alacrian Armory) and the trigger object
// (Kolodin) rather than only the Saddle keyword's Defined$ Self.
//
// The tally attributes each Saddled grant to the source permanent of the
// most recent AbilityPush/TriggerPush event (gorge's event log carries no
// source on AlterAttribute itself): only a grant from one of the three
// carriers counts, so the Mounts' own K:Saddle expansions never inflate the
// carrier count. A Mount the carrier saddled is then counted once when it is
// declared as an attacker while saddled this turn.
var ksaddletgtDecks = [][2][]cardset.Entry{{
	{
		{Name: "Guidelight Matrix", Count: 4}, {Name: "Alacrian Armory", Count: 3},
		{Name: "Kolodin, Triumph Caster", Count: 4},
		{Name: "Brightfield Mustang", Count: 4}, {Name: "Gilded Ghoda", Count: 4},
		{Name: "Bounding Felidar", Count: 2}, {Name: "Stubborn Burrowfiend", Count: 2},
		{Name: "Burner Rocket", Count: 2}, {Name: "Savannah Lions", Count: 4},
		{Name: "Raging Goblin", Count: 1},
		{Name: "Plains", Count: 5}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2},
	},
	{
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Raging Goblin", Count: 4}, {Name: "Llanowar Elves", Count: 6},
		{Name: "Giant Spider", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Forest", Count: 8}, {Name: "Mountain", Count: 6},
	},
}}

var ksaddletgtCarriers = map[string]bool{
	"Guidelight Matrix":       true,
	"Alacrian Armory":         true,
	"Kolodin, Triumph Caster": true,
}

// ksaddletgtTally records carrier-saddled Mounts and their saddled attacks.
type ksaddletgtTally struct {
	evAt    int
	src     state.ObjID
	saddles map[string]int
	saddled map[state.ObjID]bool // saddled by a carrier this turn
	seen    map[state.ObjID]int32
	attacks int
}

func (p *ksaddletgtTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.AbilityPush, events.TriggerPush:
			p.src = e.Obj
		case events.TurnChange:
			clear(p.saddled)
		case events.AlterAttribute:
			if !events.IsAlterAttribute(*e, "Saddled") || e.Amount < 1 {
				continue
			}
			if name := chName(g, p.src); ksaddletgtCarriers[name] {
				p.saddles[name]++
				p.saddled[e.Obj] = true
			}
		}
	}
	p.evAt = len(evs)
	for i := range g.Objs {
		o := &g.Objs[i]
		if o.Zone != state.ZBattlefield || !o.IsAttacking || !p.saddled[o.ID] || p.seen[o.ID] == g.Turn {
			continue
		}
		p.seen[o.ID] = g.Turn
		p.attacks++
	}
	return nil
}

// ksaddletgtPolicy is a random policy biased to attack: at an attackers
// decision it picks among the attacking options, otherwise it picks at
// random.
func ksaddletgtPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind != DAttackers {
			return rnd.Pick(c)
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActAttack {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// runSaddletgt plays the constructed matchup. mutate keeps saddled
// designations across the turn change (the mutation must diverge).
func runSaddletgt(t *testing.T, seeds int, mutate bool) (games, divs int, deaths int, saddles map[string]int, attacks int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("saddletgt")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugSaddleTgtNoClear
	dirac.DebugSaddleTgtNoClear = mutate
	defer func() { dirac.DebugSaddleTgtNoClear = old }()
	saddles = map[string]int{}
	for mi, md := range ksaddletgtDecks {
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
		for seed := uint64(1); seed <= uint64(seeds); seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"ksaddletgt-a", "ksaddletgt-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			tl := &ksaddletgtTally{
				saddles: saddles,
				saddled: map[state.ObjID]bool{},
				seen:    map[state.ObjID]int32{},
			}
			res := Lockstep(m, seed, k, ksaddletgtPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: tl.check})
			CloseSide(k)
			games++
			attacks += tl.attacks
			if res.Div != nil {
				divs++
				if !mutate {
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v",
						mi, seed, res.Div.At, res.Div.What, res.Div.Field, res.Div.Diff, res.Div.Detail,
						res.Div.RefOptions, res.Div.KerOptions)
				}
			}
			if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
				deaths++
				if !mutate {
					t.Errorf("matchup %d seed %d: %s", mi, seed, res.Stall)
				}
			}
		}
	}
	return games, divs, deaths, saddles, attacks
}

func TestSaddletgtLockstep(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, stalls, saddles, attacks := runSaddletgt(t, 256, false)
	total := 0
	for _, n := range []string{"Guidelight Matrix", "Alacrian Armory", "Kolodin, Triumph Caster"} {
		total += saddles[n]
	}
	t.Logf("%d games, %d divergences, %d kernel stalls, carrier saddles %v (total %d), saddled attacks %d",
		games, divs, stalls, saddles, total, attacks)
	if divs != 0 {
		t.Errorf("%d divergences", divs)
	}
	if stalls != 0 {
		t.Errorf("%d kernel stalls", stalls)
	}
	if total < 50 {
		t.Errorf("only %d saddles by the three carriers, want >= 50", total)
	}
	if attacks < 30 {
		t.Errorf("only %d attacks by a Mount saddled by a carrier, want >= 30", attacks)
	}
}

// TestSaddletgtMutation keeps every saddled designation into the next turn
// (kernel DebugSaddleTgtNoClear): gorge clears it at the turn change, so the
// games must diverge. A green mutation means the lockstep never reaches a
// target-saddled Mount.
func TestSaddletgtMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	games, divs, _, _, _ := runSaddletgt(t, 16, true)
	if divs == 0 {
		t.Fatalf("mutation kept saddled designations in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
