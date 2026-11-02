package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: two Count$Compare operand heads.
//
//   - Targeted$Valid Creature.withFlying: Cut Propulsion's target deals
//     twice its power to itself if it flies (Wind Drake, Serra Angel), else
//     its power;
//   - an inline ReplacedCard$CardManaCost: Thunderous Velocipede's other
//     creatures and Vehicles enter with one +1/+1 counter at MV <= 4, three
//     above (Craw Wurm, Colossal Dreadmaw), and Marketback Walker's {X}{X}
//     entrant reads its battlefield MV (no stack X; Blaze floats the mana
//     its X is paid with, kcmphPolicy).
var kcmpheadsDecks = [2][]cardset.Entry{
	{
		{Name: "Thunderous Velocipede", Count: 4}, {Name: "Cut Propulsion", Count: 4}, {Name: "Marketback Walker", Count: 4},
		{Name: "Blaze", Count: 4}, {Name: "Grizzly Bears", Count: 2}, {Name: "Hill Giant", Count: 2}, {Name: "Craw Wurm", Count: 2},
		{Name: "Colossal Dreadmaw", Count: 2}, {Name: "Forest", Count: 10}, {Name: "Mountain", Count: 6},
	},
	{
		{Name: "Wind Drake", Count: 4}, {Name: "Serra Angel", Count: 4}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Craw Wurm", Count: 3}, {Name: "Colossal Dreadmaw", Count: 2},
		{Name: "Marketback Walker", Count: 2}, {Name: "Plains", Count: 9}, {Name: "Island", Count: 8},
	},
}

// kcmphTally counts, from gorge's event log, what the pool reached.
type kcmphTally struct {
	evAt    int
	tgt     map[uint32]string // Cut Propulsion stack object -> its target's name
	marketX int               // the last Marketback Walker cast's X
	// Cut Propulsion resolutions on a flyer / not; entries under a
	// Velocipede at MV <= 4 / >= 5; Marketback Walker entries with X >= 1
	// (X >= 3: a stack-X read would make its MV 6 or more, three counters)
	cpFly, cpGround, velSmall, velBig, velMarketX, velMarketX3 int
}

func (p *kcmphTally) reset() {
	p.evAt = 0
	p.tgt = map[uint32]string{}
	p.marketX = 0
}

// kcmphVelocipede reports whether ctrl controls a Thunderous Velocipede other
// than id on the battlefield.
func kcmphVelocipede(g *state.Game, ctrl state.PlayerID, id state.ObjID) bool {
	for i := range g.Objs {
		o := &g.Objs[i]
		if state.ObjID(i+1) != id && o.Zone == state.ZBattlefield && o.Controller == ctrl && o.Face() != nil && o.Face().Name == "Thunderous Velocipede" {
			return true
		}
	}
	return false
}

func (p *kcmphTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < p.evAt {
		p.evAt = 0
	}
	for j := p.evAt; j < len(evs); j++ {
		e := &evs[j]
		switch e.Kind {
		case events.TargetsChosen:
			if chName(g, e.Obj) == "Cut Propulsion" && e.Amount == 0 && len(e.IDs) > 0 {
				p.tgt[uint32(e.Obj)] = chName(g, e.IDs[0])
			}
		case events.CastInfo:
			if chName(g, e.Obj) == "Marketback Walker" {
				p.marketX = int(e.Amount)
			}
		case events.Resolve:
			if n, ok := p.tgt[uint32(e.Obj)]; ok {
				if n == "Wind Drake" || n == "Serra Angel" {
					p.cpFly++
				} else {
					p.cpGround++
				}
				delete(p.tgt, uint32(e.Obj))
			}
		case events.MoveZone:
			if e.To != state.ZBattlefield {
				break
			}
			o := g.Obj(e.Obj)
			if o == nil || o.Face() == nil || !kcmphVelocipede(g, o.Controller, e.Obj) {
				break
			}
			switch o.Face().Name {
			case "Grizzly Bears", "Hill Giant":
				p.velSmall++
			case "Craw Wurm", "Colossal Dreadmaw":
				p.velBig++
			case "Marketback Walker":
				if e.From == state.ZStack && p.marketX >= 1 {
					p.velMarketX++
				}
				if e.From == state.ZStack && p.marketX >= 3 {
					p.velMarketX3++
				}
				p.velSmall++ // MV 0 on the battlefield
			}
		}
	}
	p.evAt = len(evs)
	return nil
}

// kcmphPolicy reaches Marketback Walker's X >= 1 entry. Gorge offers an
// {X} only the floating pool pays and an {X}{X} card at X = 0 is free (cast
// as offered, never lowered), so the walker is cast only while mana floats:
// Blaze's lowering witness taps for its largest X and Blaze announces its
// smallest, leaving the rest floating. Every other X ask takes the largest.
func kcmphPolicy(base Policy) Policy {
	xCode, _ := chooseCode("x")
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g == nil || len(c.D.Options) == 0 {
			return base.Pick(c)
		}
		if c.D.Options[0].Kind() == ActChoose+CanonKind(xCode) {
			lo, hi := c.D.Options[0], c.D.Options[0]
			for _, a := range c.D.Options {
				if a.Mode() < lo.Mode() {
					lo = a
				}
				if a.Mode() > hi.Mode() {
					hi = a
				}
			}
			if pd := g.Pending(); pd != nil && chName(g.E.G, pd.Source) == "Blaze" {
				return lo
			}
			return hi
		}
		if c.D.Kind != DPriority {
			return base.Pick(c)
		}
		floating := g.E.G.Players[c.D.Player].Pool.Total() > 0
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				if id, ok := kroleRefObj(g, a.Src()); ok && chName(g.E.G, id) == "Marketback Walker" {
					if floating {
						return a
					}
					continue
				}
			}
			buf = append(buf, a)
		}
		if len(buf) == len(c.D.Options) || len(buf) == 0 {
			return base.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return base.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// kcmphRun plays seeds 1..seeds; stopAtDiv ends it at the first divergence.
func kcmphRun(t *testing.T, seeds uint64, stopAtDiv bool) (tally *kcmphTally, games, divs, stalls int, first string) {
	t.Helper()
	reg, set, tbl, decks := kcmphSetup(t)
	tally = &kcmphTally{}
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%4 == 0}
		m, err := NewMatchup(reg, [2]string{"kch-a", "kch-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		tally.reset()
		pol := kcmphPolicy(castFirstPolicy(seed))
		if seed%8 == 0 {
			pol = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40, Check: tally.check})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !stopAtDiv {
				t.Errorf("seed %d: kernel stall %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if first == "" {
				first = "seed " + strconv.FormatUint(seed, 10) + ": " + d.What + " " + d.Field
			}
			lo := max(d.At-60, 0)
			t.Logf("seed %d %s %s at %d last actions %v\ndiff %v %s\nref %v\nker %v", seed, d.What, d.Field, d.At, actionStrings(res.Actions[max(lo, d.At-8):d.At]), d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			if stopAtDiv {
				return
			}
		}
	}
	return
}

func TestCmpheadsLockstep(t *testing.T) {
	t.Parallel()
	tally, games, divs, stalls, first := kcmphRun(t, 300, false)
	t.Logf("%d games, %d divergences, %d stalls; Cut Propulsion resolutions: flyer %d, non-flyer %d; entries under a Velocipede: MV<=4 %d, MV>=5 %d, Marketback X>=1 %d (X>=3 %d)",
		games, divs, stalls, tally.cpFly, tally.cpGround, tally.velSmall, tally.velBig, tally.velMarketX, tally.velMarketX3)
	if divs != 0 {
		t.Fatalf("%d divergences, first %s", divs, first)
	}
	if stalls != 0 {
		t.Fatalf("%d stalls", stalls)
	}
	for _, c := range []struct {
		name      string
		got, want int
	}{
		{"Cut Propulsion resolutions", tally.cpFly + tally.cpGround, 20},
		{"Cut Propulsion resolutions on a flyer", tally.cpFly, 5},
		{"Cut Propulsion resolutions on a non-flyer", tally.cpGround, 5},
		{"entries under a Velocipede", tally.velSmall + tally.velBig, 20},
		{"entries under a Velocipede at MV <= 4", tally.velSmall, 5},
		{"entries under a Velocipede at MV >= 5", tally.velBig, 5},
		{"Marketback Walker X >= 1 entries under a Velocipede", tally.velMarketX, 3},
		{"Marketback Walker X >= 3 entries under a Velocipede", tally.velMarketX3, 1},
	} {
		if c.got < c.want {
			t.Errorf("%s: %d, want at least %d", c.name, c.got, c.want)
		}
	}
}

// TestCmpheadsMutation: with every ExMVOf of the replaced object read as 0,
// a Velocipede's big entrant gets one counter instead of three and the
// lockstep must diverge within 128 seeds.
func TestCmpheadsMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugMVOfReplacedZero = true
	defer func() { dirac.DebugMVOfReplacedZero = false }()
	_, games, divs, _, first := kcmphRun(t, 128, true)
	if divs == 0 {
		t.Fatalf("no divergence in %d games with the replaced object's MV read as 0", games)
	}
	t.Logf("mutation diverged after %d games: %s", games, first)
}

// kcmphSetup loads the corpus, the generated set and its matchup table.
func kcmphSetup(t *testing.T) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("cmpheads")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kcmpheadsDecks {
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
	tbl, _, _, err := set.Matchup(kcmpheadsDecks[0], kcmpheadsDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}
