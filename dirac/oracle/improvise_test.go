package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The improvise lockstep (CR 702.126): constructed matchups over the
// generated improvise pool, so every improvise_generic announcement gorge
// poses is answered by the kernel's bsConvoke ask (mode-6 picks). The frozen
// FDN and BLB pair pools carry no improvise card.

var improviseDecks = [][2][]cardset.Entry{{
	{
		{Name: "Ornithopter", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Maverick Thopterist", Count: 6}, {Name: "Reverse Engineer", Count: 6}, {Name: "Foundry Assembler", Count: 4},
		{Name: "Arc Reactor", Count: 2}, {Name: "Island", Count: 7}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Ornithopter", Count: 4}, {Name: "Sly Requisitioner", Count: 4}, {Name: "Sweatworks Brawler", Count: 4},
		{Name: "Arc Reactor", Count: 2}, {Name: "Foundry Assembler", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Enraged Giant", Count: 4}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 7},
	},
}, {
	{
		{Name: "Ornithopter", Count: 4}, {Name: "Sly Requisitioner", Count: 4}, {Name: "Sweatworks Brawler", Count: 4},
		{Name: "Arc Reactor", Count: 3}, {Name: "Mind Stone", Count: 4}, {Name: "Enraged Giant", Count: 5},
		{Name: "Foundry Assembler", Count: 2}, {Name: "Swamp", Count: 7}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Ornithopter", Count: 4}, {Name: "Mind Stone", Count: 4},
		{Name: "Maverick Thopterist", Count: 6}, {Name: "Reverse Engineer", Count: 6}, {Name: "Foundry Assembler", Count: 4},
		{Name: "Island", Count: 10}, {Name: "Mountain", Count: 6},
	},
}, {
	// Artifacts ahead of the lands: a declined Foundry Assembler's window taps every
	// land, floating the coloured pips; gorge offers these two with the lands
	// untapped too, whenever the pool plus the sources pay what the credit leaves.
	{
		{Name: "Ornithopter", Count: 18}, {Name: "Foundry Assembler", Count: 8},
		{Name: "Reverse Engineer", Count: 8}, {Name: "Island", Count: 6},
	},
	{
		{Name: "Ornithopter", Count: 18}, {Name: "Foundry Assembler", Count: 8},
		{Name: "Enraged Giant", Count: 8}, {Name: "Mountain", Count: 6},
	},
}}

const improviseFloatMatchup = 2

var improviseCarriers = map[string]bool{
	"Maverick Thopterist": true, "Reverse Engineer": true, "Foundry Assembler": true,
	"Arc Reactor": true, "Sly Requisitioner": true, "Sweatworks Brawler": true, "Enraged Giant": true,
}

type improviseStats struct {
	casts, zero, picks, multi, creatureTap, nonCreatureTap, sick int
	// per carrier: improvise announcements, and announcements with a pick
	announce, picked map[string]int
}

type improviseProbe struct {
	st       *improviseStats
	base     Policy
	lastCast string
	n        int
	asking   bool
	// declines: answer the common carriers' announcements with no artifact. A cast
	// the lands can't pay then taps them in the window and reverses, leaving their
	// mana floating; a carrier gorge's plan prices is offered with the pool empty
	// too, when its untapped sources pay what the credit leaves.
	declines bool
	// castsFloated: cast Reverse Engineer or Enraged Giant the moment it's offered.
	// Only floated mana offers one, and the bot rarely ranks it first.
	castsFloated bool
}

func (p *improviseProbe) finish() {
	if !p.asking {
		return
	}
	p.asking = false
	p.st.casts++
	if p.st.announce == nil {
		p.st.announce, p.st.picked = map[string]int{}, map[string]int{}
	}
	p.st.announce[p.lastCast]++
	if p.n > 0 {
		p.st.picked[p.lastCast]++
	}
	if p.n == 0 {
		p.st.zero++
	}
	if p.n >= 2 {
		p.st.multi++
	}
	p.n = 0
}

func (p *improviseProbe) pick(c *PickCtx) Canon {
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
	} else {
		p.asking = true
	}
	a := p.base.Pick(c)
	if !isAsk && p.castsFloated && c.G != nil {
		for _, x := range c.D.Options {
			if x.Kind() != ActCast {
				continue
			}
			if o, ok := c.G.GorgeOption(x); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && (ob.Face().Name == "Reverse Engineer" || ob.Face().Name == "Enraged Giant") {
					a = x
					break
				}
			}
		}
	}
	if p.asking && p.declines && (p.lastCast == "Arc Reactor" || p.lastCast == "Foundry Assembler") {
		for _, o := range c.D.Options {
			if o.Kind() == ActDone {
				a = o
			}
		}
	}
	if a.Kind() == ActCast && c.G != nil {
		if o, ok := c.G.GorgeOption(a); ok {
			if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
				p.lastCast = ob.Face().Name
				if !improviseCarriers[p.lastCast] {
					p.lastCast = ""
				}
			}
		}
	}
	if p.asking && a.Kind() == kind {
		if o, ok := c.G.GorgeOption(a); ok && o.Kind == "improvise_generic" {
			p.n++
			p.st.picks++
			if ob := c.G.E.G.Obj(o.Obj); ob != nil {
				if ob.EffectiveIsCreature() {
					p.st.creatureTap++
				} else {
					p.st.nonCreatureTap++
				}
				if ob.SummonSick {
					p.st.sick++
				}
			}
		}
	}
	return a
}

func improviseSetup(t *testing.T, md [2][]cardset.Entry) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("improvise")
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

func TestImproviseLockstep(t *testing.T) {
	t.Parallel()

	st := &improviseStats{}
	games := 0
	var turns [8]int
	for mi, md := range improviseDecks {
		reg, set, tbl, decks := improviseSetup(t, md)
		floats := mi == improviseFloatMatchup
		seeds := uint64(400)
		if floats {
			seeds = 60 // a game repeats its declined casts, so these clear the floors many times over
		}
		for seed := uint64(1); seed <= seeds; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"improvise-a", "improvise-b"}, decks, 1)
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
			probe := &improviseProbe{st: st, base: base, declines: floats || seed%3 == 1, castsFloated: floats}
			res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			probe.finish()
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: %s", mi, seed, res.Stall)
			}
			turns[mi] += int(res.Turns)
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
	t.Logf("%d games, improvise %+v, turns per matchup %v", games, *st, turns)
	for _, n := range []string{"Maverick Thopterist", "Reverse Engineer", "Foundry Assembler",
		"Arc Reactor", "Sly Requisitioner", "Sweatworks Brawler", "Enraged Giant"} {
		if st.picked[n] < 3 {
			t.Errorf("%s improvised an artifact in fewer than 3 casts (announcements %d)", n, st.announce[n])
		}
	}
	for _, c := range []struct {
		n int
		s string
	}{
		{st.casts, "an improvise announcement"},
		{st.picks, "an improvise_generic pick"},
		{st.multi, "a cast improvising two or more artifacts"},
		{st.zero, "an improvise announcement answered with no pick"},
		{st.creatureTap, "an artifact creature improvised"},
		{st.nonCreatureTap, "a noncreature artifact improvised"},
		{st.sick, "a summoning-sick artifact improvising"},
	} {
		if c.n == 0 {
			t.Errorf("%s never observed", c.s)
		}
	}
}

// improviseDiverge runs seeds of the matchup under the random policy (which
// picks announcements beyond what the cost absorbs) and reports whether any
// game diverged.
func improviseDiverge(t *testing.T, md [2][]cardset.Entry, seeds uint64) bool {
	t.Helper()
	reg, set, tbl, decks := improviseSetup(t, md)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"improvise-mut-a", "improvise-mut-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, RandomPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
		CloseSide(k)
		if res.Div != nil {
			return true
		}
	}
	return false
}

// TestImproviseMutation: tapping the picked artifacts when they are picked
// instead of at payment must make the lockstep diverge. (Convoke's second
// mutation, dropping the oracle's absorption predicate, has no improvise
// counterpart: gorge's planner refuses a Convoke/Improvise cost, so a cast is
// offered only when the pool plus the greedy credit pays it, and improvise
// credits generic only, so every offered carrier is generic-only and its
// announcement never has a slot an artifact cannot absorb.)
func TestImproviseMutation(t *testing.T) {
	dirac.DebugConvokeTapAtPick = true
	defer func() { dirac.DebugConvokeTapAtPick = false }()
	for _, md := range improviseDecks {
		if improviseDiverge(t, md, 384) {
			return
		}
	}
	t.Fatal("tap at pick: no divergence in 384 seeds per matchup")
}
