package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Effect-delivered spell-flash lockstep (gorge effectFlashGrants and
// castSorceryTimingExempt): Borne Upon a Wind, Quicken, Alchemist's Refuge,
// Emergence Zone, Winding Canyons and Teferi, Time Raveler's +1 grant "you may
// cast ... spells as though they had flash" through an Effect's
// StaticAbilities$ (an emblem in the kernel); Quicken's ForgetOnCast$ ends it
// at the next sorcery cast, and Teferi's static lets an opponent cast only at
// sorcery timing (OnlySorcerySpeed$). A-Teferi's static is the plain
// "can't cast during your turn". Complete the Circuit pairs the grant with a
// delayed copy trigger and stays refused: no lockstep game casts it.

var effectFlashDecks = [2][]cardset.Entry{
	{ // A: the grants beside sorceries, creatures and instants
		{Name: "Borne Upon a Wind", Count: 3}, {Name: "Quicken", Count: 3}, {Name: "Teferi, Time Raveler", Count: 2},
		{Name: "A-Teferi, Time Raveler", Count: 1},
		{Name: "Alchemist's Refuge", Count: 2}, {Name: "Emergence Zone", Count: 2}, {Name: "Winding Canyons", Count: 2},
		{Name: "Mulch", Count: 2}, {Name: "Accumulate Wisdom", Count: 2}, {Name: "Environmental Sciences", Count: 1},
		{Name: "Cloud Sprite", Count: 2}, {Name: "Grizzly Bears", Count: 2}, {Name: "Elite Vanguard", Count: 1},
		{Name: "Cancel", Count: 1}, {Name: "Giant Growth", Count: 1},
		{Name: "Island", Count: 5}, {Name: "Forest", Count: 4}, {Name: "Plains", Count: 4},
	},
	{ // B: instants to hold under Teferi, creatures and sorceries
		{Name: "Swords to Plowshares", Count: 3}, {Name: "Raise the Alarm", Count: 3}, {Name: "Cancel", Count: 3}, {Name: "Giant Growth", Count: 2},
		{Name: "Glory Seeker", Count: 3}, {Name: "Elite Vanguard", Count: 2}, {Name: "Border Guard", Count: 2}, {Name: "Cloud Sprite", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Mulch", Count: 2},
		{Name: "Island", Count: 6}, {Name: "Plains", Count: 5}, {Name: "Forest", Count: 5},
	},
}

// effectFlashQuickenDecks is the matchup built to reach Quicken's end: A holds
// five Quickens and a dozen cheap sorceries, so a second sorcery is often in
// hand and payable the turn the first one ended the grant.
var effectFlashQuickenDecks = [2][]cardset.Entry{
	{
		{Name: "Quicken", Count: 5}, {Name: "Mulch", Count: 4}, {Name: "Accumulate Wisdom", Count: 4}, {Name: "Environmental Sciences", Count: 4},
		{Name: "Cloud Sprite", Count: 3}, {Name: "Grizzly Bears", Count: 2}, {Name: "Cancel", Count: 2}, {Name: "Giant Growth", Count: 1},
		{Name: "Island", Count: 8}, {Name: "Forest", Count: 7},
	},
	effectFlashDecks[1],
}

type effectFlashStats struct {
	offTiming   int // off-sorcery-timing casts of a card with no flash of its own (a sorcery or a creature)
	grantUses   map[string]int
	grantMade   map[string]int // grant sources activated or cast
	underTeferi int            // priority windows of an opponent of a Teferi controller with a cast offered
	withheld    int            // ... windows where an instant in hand was not offered off sorcery timing
}

type effectFlashProbe struct {
	st   *effectFlashStats
	base Policy
	r    *rand.Rand
}

func faceHasKeyword(f *cards.Face, kw string) bool {
	for _, k := range f.Keywords {
		if k == kw {
			return true
		}
	}
	return false
}

var effectFlashSources = map[string]bool{
	"Borne Upon a Wind": true, "Quicken": true, "Teferi, Time Raveler": true,
	"Alchemist's Refuge": true, "Emergence Zone": true, "Winding Canyons": true,
	"A-Teferi, Time Raveler": true,
}

func (p *effectFlashProbe) pick(c *PickCtx) Canon {
	a := p.base.Pick(c)
	if c.G == nil {
		return a
	}
	d := c.G.Pending()
	if d == nil || d.Kind != "priority" {
		return a
	}
	g := c.G.E.G
	who := g.Priority
	off := !(g.Step.IsMain() && g.Active == who && len(g.Stack) == 0)
	teferi := false
	for _, id := range g.Zone(state.ZBattlefield, 1-who) {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Teferi, Time Raveler" {
			teferi = true
		}
	}
	var prefer []Canon // the grant sources, and off-timing casts of cards with no flash of their own
	offeredID := map[state.ObjID]bool{}
	for _, x := range c.D.Options {
		o, ok := c.G.GorgeOption(x)
		if !ok {
			continue
		}
		ob := g.Obj(o.Obj)
		if ob == nil || ob.Face() == nil {
			continue
		}
		f := ob.Face()
		switch {
		case x.Kind() == ActCast && off && !hasFaceType(f, "Instant") && !faceHasKeyword(f, "Flash"):
			prefer = append(prefer, x)
			offeredID[o.Obj] = true
		case (x.Kind() == ActCast || x.Kind() == ActActivate) && effectFlashSources[f.Name]:
			prefer = append(prefer, x)
		}
		if x.Kind() == ActCast {
			offeredID[o.Obj] = true
		}
	}
	if teferi {
		p.st.underTeferi++
		if off {
			for _, id := range g.Zone(state.ZHand, who) {
				if ob := g.Obj(id); ob != nil && ob.Face() != nil && hasFaceType(ob.Face(), "Instant") && !offeredID[id] {
					p.st.withheld++
					break
				}
			}
		}
	}
	if len(prefer) > 0 && p.r.IntN(4) != 0 {
		a = prefer[p.r.IntN(len(prefer))]
	}
	if o, ok := c.G.GorgeOption(a); ok {
		if ob := g.Obj(o.Obj); ob != nil && ob.Face() != nil {
			f := ob.Face()
			if a.Kind() == ActCast && off && !hasFaceType(f, "Instant") && !faceHasKeyword(f, "Flash") {
				p.st.offTiming++
				if p.st.grantUses == nil {
					p.st.grantUses = map[string]int{}
				}
				p.st.grantUses[firstType(f)]++
			}
			if effectFlashSources[f.Name] {
				if p.st.grantMade == nil {
					p.st.grantMade = map[string]int{}
				}
				p.st.grantMade[f.Name]++
			}
		}
	}
	return a
}

func firstType(f *cards.Face) string {
	for _, t := range []string{"Creature", "Sorcery"} {
		if hasFaceType(f, t) {
			return t
		}
	}
	return "other"
}

func effectFlashSetup(t *testing.T, pair [2][]cardset.Entry) (*cards.Registry, cardset.Set, *kop.Table, [2][]*cards.Card) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effectflash")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range pair {
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
	tbl, _, _, err := set.Matchup(pair[0], pair[1])
	if err != nil {
		t.Fatal(err)
	}
	return reg, set, tbl, decks
}

func effectFlashRun(t *testing.T, pair [2][]cardset.Entry, seeds uint64, report bool) (diverged bool, stalls int, st effectFlashStats) {
	t.Helper()
	reg, set, tbl, decks := effectFlashSetup(t, pair)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"effectflash-a", "effectflash-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &effectFlashProbe{st: &st, base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6566666c))}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 24})
		CloseSide(k)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
		}
		if d := res.Div; d != nil {
			diverged = true
			if report {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			return
		}
	}
	return
}

func TestEffectflashLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := effectFlashRun(t, effectFlashDecks, 256, true)
	t.Logf("256 games, %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	for _, name := range []string{"Borne Upon a Wind", "Quicken", "Teferi, Time Raveler", "Alchemist's Refuge", "Emergence Zone", "Winding Canyons", "A-Teferi, Time Raveler"} {
		if st.grantMade[name] < 3 {
			t.Errorf("%s: %d plays or activations, want at least 3", name, st.grantMade[name])
		}
	}
	for _, c := range []struct {
		n, min int
		s      string
	}{
		{st.grantUses["Sorcery"], 10, "off-timing sorcery casts"},
		{st.grantUses["Creature"], 10, "off-timing creature casts"},
		{st.underTeferi, 20, "opponent priority windows under Teferi"},
		{st.withheld, 5, "instants withheld from an opponent under Teferi off sorcery timing"},
	} {
		if c.n < c.min {
			t.Errorf("%s: %d, want at least %d", c.s, c.n, c.min)
		}
	}
}

// TestEffectflashMutation: each of the three kernel reads switched off must
// make the lockstep diverge (gorge offers the grant, the ForgetOnCast end or
// the sorcery-timing cast; the kernel does not).
func TestEffectflashQuickenLockstep(t *testing.T) {
	t.Parallel()
	div, stalls, st := effectFlashRun(t, effectFlashQuickenDecks, 128, true)
	t.Logf("128 games, %+v, %d divergences, %d stalls", st, b2i(div), stalls)
	if stalls != 0 {
		t.Errorf("%d stalled games", stalls)
	}
	if st.grantUses["Sorcery"] < 20 {
		t.Errorf("off-timing sorcery casts: %d, want at least 20", st.grantUses["Sorcery"])
	}
}

func TestEffectflashMutation(t *testing.T) {
	for _, m := range []struct {
		name string
		sw   *bool
		pair [2][]cardset.Entry
	}{
		{"DebugEmbFlashOff", &dirac.DebugEmbFlashOff, effectFlashDecks},
		{"DebugEmbForgetOff", &dirac.DebugEmbForgetOff, effectFlashQuickenDecks},
		{"DebugCantSorceryOff", &dirac.DebugCantSorceryOff, effectFlashDecks},
	} {
		*m.sw = true
		diverged, _, _ := effectFlashRun(t, m.pair, 128, false)
		*m.sw = false
		if !diverged {
			t.Errorf("%s: no divergence in 128 seeds", m.name)
		}
	}
}
