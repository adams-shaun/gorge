package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The convoke lockstep: constructed go-wide matchups over the generated
// convoke pool, so every convoke announcement gorge poses (CR 702.51) is
// answered by the kernel's bsConvoke ask. The frozen FDN and BLB pair pools
// carry no convoke card, so they exercise none of these arms.

var convokeDecks = [][2][]cardset.Entry{{
	{
		{Name: "Aerial Boost", Count: 4}, {Name: "Protective Response", Count: 4}, {Name: "Vote Out", Count: 4},
		{Name: "Halo Hopper", Count: 3}, {Name: "Grizzly Bears", Count: 4}, {Name: "Giant Spider", Count: 3},
		{Name: "Plains", Count: 12}, {Name: "Forest", Count: 6},
	},
	{
		{Name: "Burning Sun's Fury", Count: 4}, {Name: "Stoke the Flames", Count: 4}, {Name: "Collective Nightmare", Count: 4},
		{Name: "Merrow Skyswimmer", Count: 3}, {Name: "Gray Ogre", Count: 4}, {Name: "Walking Corpse", Count: 3},
		{Name: "Mountain", Count: 8}, {Name: "Island", Count: 4}, {Name: "Swamp", Count: 6},
	},
}, {
	{
		{Name: "Stoke the Flames", Count: 4}, {Name: "Collective Nightmare", Count: 4}, {Name: "Burning Sun's Fury", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Walking Corpse", Count: 4}, {Name: "Merrow Skyswimmer", Count: 2},
		{Name: "Mountain", Count: 8}, {Name: "Swamp", Count: 6}, {Name: "Island", Count: 4},
	},
	{
		{Name: "Aerial Boost", Count: 4}, {Name: "Vote Out", Count: 4}, {Name: "Protective Response", Count: 4},
		{Name: "Halo Hopper", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Gray Ogre", Count: 4},
		{Name: "Plains", Count: 8}, {Name: "Forest", Count: 8},
	},
}}

// convokeMV is the mana value of each carrier (the convoke announcement's
// slot count: coloured pips plus generic).
var convokeMV = map[string]int{
	"Aerial Boost": 2, "Burning Sun's Fury": 2, "Collective Nightmare": 3,
	"Halo Hopper": 3, "Merrow Skyswimmer": 5, "Protective Response": 3,
	"Stoke the Flames": 4, "Vote Out": 4,
}

type convokeStats struct {
	casts, zero, partial, full, colored, generic, sick int
}

type convokeCast struct {
	name                   string
	picks                  int
	colored, generic, sick bool
}

type convokeProbe struct {
	st       *convokeStats
	base     Policy
	lastCast string
	cur      *convokeCast
}

func (p *convokeProbe) finish() {
	c := p.cur
	if c == nil {
		return
	}
	p.cur = nil
	p.st.casts++
	if c.colored {
		p.st.colored++
	}
	if c.generic {
		p.st.generic++
	}
	if c.sick {
		p.st.sick++
	}
	switch {
	case c.picks == 0:
		p.st.zero++
	case c.picks >= convokeMV[c.name]:
		p.st.full++
	default:
		p.st.partial++
	}
}

func (p *convokeProbe) pick(c *PickCtx) Canon {
	cp, _ := chooseCode("convoke")
	kind := ActChoose + CanonKind(cp)
	isAsk := false
	for _, a := range c.D.Options {
		if a.Kind() == kind {
			isAsk = true
			break
		}
	}
	if !isAsk {
		p.finish()
	}
	if isAsk && p.cur == nil {
		p.cur = &convokeCast{name: p.lastCast}
	}
	a := p.base.Pick(c)
	if a.Kind() == ActCast && c.G != nil {
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				if _, ok := convokeMV[ob.Face().Name]; ok {
					p.lastCast = ob.Face().Name
				} else {
					p.lastCast = ""
				}
			}
		}
	}
	if p.cur != nil && a.Kind() == kind {
		if o, ok := c.G.GorgeOption(a); ok {
			p.cur.picks++
			if o.Kind == "convoke_generic" {
				p.cur.generic = true
			} else {
				p.cur.colored = true
			}
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.SummonSick {
				p.cur.sick = true
			}
		}
	}
	return a
}

func TestConvokeLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("convoke")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	st := &convokeStats{}
	// 1024 games total (512 seeds per matchup), keeping the test under its
	// 60 s budget; the per-category occurrence minima below are measured
	// over that run.
	lo, hi := uint64(1), uint64(512)
	for mi, md := range convokeDecks {
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
		for seed := lo; seed <= hi; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"convoke-a", "convoke-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			base := Policy(HeuristicPolicy(seed))
			if seed%3 == 0 {
				base = RandomPolicy(seed)
			}
			probe := &convokeProbe{st: st, base: base}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			probe.finish()
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
	t.Logf("%d games, convoke %+v", games, *st)
	for _, c := range []struct {
		n int
		s string
	}{
		{st.casts, "convoke casts"},
		{st.zero, "a convoke cast with no pick"},
		{st.partial, "a partial convoke payment"},
		{st.full, "a fully creature-paid cast"},
		{st.colored, "a coloured-pip pick"},
		{st.generic, "a generic pick"},
		{st.sick, "a summoning-sick creature convoking"},
	} {
		if c.n == 0 {
			t.Errorf("%s never observed", c.s)
		}
	}
}

// convokeSetup builds one matchup's corpus decks and table.
func convokeSetup(t *testing.T, md [2][]cardset.Entry) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("convoke")
	if err != nil {
		t.Fatal(err)
	}
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
	return reg, set, tbl, decks
}

// convokeDiverge runs seeds and reports whether any game diverged.
func convokeDiverge(t *testing.T, reg *cards.Registry, set cardset.Set, tbl *kop.Table, decks [2][]*cards.Card, seeds uint64) bool {
	t.Helper()
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"convoke-mut-a", "convoke-mut-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		base := Policy(HeuristicPolicy(seed))
		if seed%3 == 0 {
			base = RandomPolicy(seed)
		}
		res := Lockstep(m, seed, k, base, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			return true
		}
	}
	return false
}

// TestConvokeMutation: tapping the picked creatures when they are picked
// instead of at payment, and dropping the oracle's absorption predicate,
// must each make the lockstep diverge.
func TestConvokeMutation(t *testing.T) {
	reg, set, tbl, decks := convokeSetup(t, convokeDecks[0])
	for _, c := range []struct {
		name string
		set  func(bool)
	}{
		{"tap at pick", func(on bool) { dirac.DebugConvokeTapAtPick = on }},
		{"no absorb", func(on bool) { dirac.DebugConvokeNoAbsorb = on }},
	} {
		t.Run(c.name, func(t *testing.T) {
			c.set(true)
			defer c.set(false)
			if !convokeDiverge(t, reg, set, tbl, decks, 64) {
				t.Fatalf("%s: no divergence in 64 seeds", c.name)
			}
		})
	}
}
