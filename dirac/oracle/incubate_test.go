package oracle

import (
	"sync"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The Incubate lockstep (CR 701.53a-b): the MOM carriers whose
// api:Incubate lowers, Doubling Season and basics. The decks spread the
// carriers so every one is cast or activated; the policy also counts the
// Incubator token's {2}: Transform (CR 701.27a) and the tokens minted under
// Doubling Season.
var incubateDecks = [][2][]cardset.Entry{
	{
		{
			{Name: "Converter Beast", Count: 4}, {Name: "Gift of Compleation", Count: 4},
			{Name: "Compleated Huntmaster", Count: 4}, {Name: "Glistening Dawn", Count: 4},
			{Name: "Tangled Skyline", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Swamp", Count: 10},
		},
		{
			{Name: "Converter Beast", Count: 4}, {Name: "Gift of Compleation", Count: 4},
			{Name: "Compleated Huntmaster", Count: 4}, {Name: "Glistening Dawn", Count: 4},
			{Name: "Tangled Skyline", Count: 4}, {Name: "Forest", Count: 10}, {Name: "Swamp", Count: 10},
		},
	},
	{
		{
			{Name: "Essence of Orthodoxy", Count: 4}, {Name: "Phyrexian Awakening", Count: 4},
			{Name: "Sculpted Perfection", Count: 4}, {Name: "Infected Defector", Count: 4},
			{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
		},
		{
			{Name: "Essence of Orthodoxy", Count: 4}, {Name: "Phyrexian Awakening", Count: 4},
			{Name: "Sculpted Perfection", Count: 4}, {Name: "Infected Defector", Count: 4},
			{Name: "Plains", Count: 12}, {Name: "Swamp", Count: 12},
		},
	},
	{
		{
			{Name: "Chrome Host Seedshark", Count: 4}, {Name: "Eyes of Gitaxias", Count: 4},
			{Name: "Furnace Gremlin", Count: 4}, {Name: "Marauding Dreadship", Count: 4},
			{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
		},
		{
			{Name: "Chrome Host Seedshark", Count: 4}, {Name: "Eyes of Gitaxias", Count: 4},
			{Name: "Furnace Gremlin", Count: 4}, {Name: "Marauding Dreadship", Count: 4},
			{Name: "Island", Count: 12}, {Name: "Mountain", Count: 12},
		},
	},
	{
		{
			{Name: "Tiller of Flesh", Count: 4}, {Name: "Traumatic Revelation", Count: 4},
			{Name: "Merciless Repurposing", Count: 4}, {Name: "Injector Crocodile", Count: 4},
			{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 12},
		},
		{
			{Name: "Tiller of Flesh", Count: 4}, {Name: "Traumatic Revelation", Count: 4},
			{Name: "Merciless Repurposing", Count: 4}, {Name: "Injector Crocodile", Count: 4},
			{Name: "Swamp", Count: 12}, {Name: "Plains", Count: 12},
		},
	},
	{
		{
			{Name: "Bloated Processor", Count: 4}, {Name: "Phyrexian Walker", Count: 4},
			{Name: "Doubling Season", Count: 4}, {Name: "Swamp", Count: 14}, {Name: "Forest", Count: 14},
		},
		{
			{Name: "Bloated Processor", Count: 4}, {Name: "Phyrexian Walker", Count: 4},
			{Name: "Doubling Season", Count: 4}, {Name: "Swamp", Count: 14}, {Name: "Forest", Count: 14},
		},
	},
	{
		{
			{Name: "Brimaz, Blight of Oreskos", Count: 4}, {Name: "Excise the Imperfect", Count: 4},
			{Name: "Blight Titan", Count: 4}, {Name: "Ichor Drinker", Count: 4},
			{Name: "Phyrexian Walker", Count: 4}, {Name: "Infected Defector", Count: 4},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
		},
		{
			{Name: "Brimaz, Blight of Oreskos", Count: 4}, {Name: "Excise the Imperfect", Count: 4},
			{Name: "Blight Titan", Count: 4}, {Name: "Ichor Drinker", Count: 4},
			{Name: "Phyrexian Walker", Count: 4}, {Name: "Infected Defector", Count: 4},
			{Name: "Plains", Count: 8}, {Name: "Swamp", Count: 8},
		},
	},
}

// incubateCarriers names the cards whose Incubate resolution the test counts
// (each must resolve at least once, not merely be cast or activated). Glissa
// and Sunder the Gateway are not in the pool: their targeted SetState
// Transform stays refused.
var incubateCarriers = []string{
	"Blight Titan", "Bloated Processor", "Brimaz, Blight of Oreskos", "Chrome Host Seedshark",
	"Compleated Huntmaster", "Converter Beast",
	"Essence of Orthodoxy", "Excise the Imperfect", "Eyes of Gitaxias", "Furnace Gremlin", "Gift of Compleation",
	"Glistening Dawn", "Ichor Drinker", "Infected Defector", "Injector Crocodile",
	"Marauding Dreadship", "Merciless Repurposing", "Phyrexian Awakening", "Sculpted Perfection",
	"Tangled Skyline", "Tiller of Flesh", "Traumatic Revelation",
}

type incubateStats struct {
	carriers   map[string]int // Incubate resolutions, by carrier name
	transforms int
	times      int
	doubled    int
	lastTokens int
	lastDS     bool
}

// incubateSA reports whether sa, or one of its SubAbility$ chain, is
// api:Incubate. A carrier's Incubate can be the whole ability (Brimaz, Ichor
// Drinker) or a SubAbility$ of another effect (Blight Titan's mill, Excise's
// exile).
func incubateSA(sa *cards.SA) bool {
	for ; sa != nil; sa = sa.Sub {
		if sa.API == "Incubate" {
			return true
		}
	}
	return false
}

func incubateTokens(c *PickCtx) (tokens int, ds bool) {
	for p := state.PlayerID(0); p < 2; p++ {
		for _, id := range c.G.E.G.Zone(state.ZBattlefield, p) {
			ob := c.G.E.G.Obj(id)
			if ob == nil || ob.Face() == nil {
				continue
			}
			switch ob.Face().Name {
			case "Doubling Season":
				ds = true
			case "Incubator Token", "Phyrexian Token":
				tokens++
			}
		}
	}
	return tokens, ds
}

func otherPhyrexian(c *PickCtx, ctrl state.PlayerID) bool {
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, ctrl) {
		ob := c.G.E.G.Obj(id)
		if ob == nil || ob.Face() == nil {
			continue
		}
		for _, ty := range ob.Face().Types {
			if ty == "Phyrexian" {
				return true
			}
		}
	}
	return false
}

func incubatePolicy(seed uint64, st *incubateStats) Policy {
	rnd := RandomPolicy(seed)
	carrier := map[string]bool{}
	for _, n := range incubateCarriers {
		carrier[n] = true
	}
	// pending maps an Incubate-carrying stack object to its carrier name.
	// It counts a RESOLUTION (the stack object leaving) rather than the
	// cast/activation that put it there, so a dies trigger that never
	// resolves does not count.
	pending := map[state.ObjID]string{}
	return PolicyFunc(func(c *PickCtx) Canon {
		onStack := map[state.ObjID]bool{}
		for _, sid := range c.G.E.G.Stack {
			ob := c.G.E.G.Obj(sid)
			if ob == nil {
				continue
			}
			sa := ob.Ability
			if sa == nil && ob.Face() != nil {
				sa = ob.Face().SpellAbility() // a spell on the stack: no Ability, its face's spell
			}
			if !incubateSA(sa) {
				continue
			}
			onStack[sid] = true
			if _, ok := pending[sid]; ok {
				continue
			}
			name := ""
			if src := c.G.E.G.Obj(ob.Source); src != nil && src.Face() != nil {
				name = src.Face().Name
			} else if ob.Face() != nil {
				name = ob.Face().Name
			}
			pending[sid] = name
		}
		for sid, name := range pending {
			if onStack[sid] {
				continue
			}
			if carrier[name] {
				st.carriers[name]++
			}
			delete(pending, sid)
		}
		tokens, ds := incubateTokens(c)
		if c.Seq <= 1 {
			st.lastTokens, st.lastDS = tokens, ds
		}
		if tokens > st.lastTokens {
			if st.lastDS {
				st.doubled += tokens - st.lastTokens
			}
			st.lastTokens = tokens
		}
		st.lastDS = ds
		a := rnd.Pick(c)
		k := a.Kind()
		if k == ActCast {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Bloated Processor" && !otherPhyrexian(c, ob.Controller) {
					return Pack(ActPass, RefNone, RefNone, 0)
				}
			}
		}
		if k == ActCast || k == ActActivate {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					name := ob.Face().Name
					if k == ActActivate && name == "Incubator Token" {
						st.transforms++
					}
					if k == ActCast && name == "Glistening Dawn" {
						st.times++
					}
				}
			}
		}
		return a
	})
}

// incubatePrep is one matchup's decks and kernel table, built once: the
// table is read-only across games, so 1024 games do not rebuild it.
type incubatePrep struct {
	md    [2][]cardset.Entry
	decks [2][]*cards.Card
	tbl   *kop.Table
}

func prepareIncubate(t *testing.T, reg *cards.Registry, set cardset.Set) []incubatePrep {
	t.Helper()
	out := make([]incubatePrep, len(incubateDecks))
	for mi, md := range incubateDecks {
		p := incubatePrep{md: md}
		for s, d := range md {
			for _, e := range d {
				c, ok := reg.Lookup(e.Name)
				if !ok {
					t.Fatalf("%s not in the corpus", e.Name)
				}
				for j := 0; j < e.Count; j++ {
					p.decks[s] = append(p.decks[s], c)
				}
			}
			if len(p.decks[s]) != 40 {
				t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(p.decks[s]))
			}
		}
		tbl, _, _, err := set.Matchup(md[0], md[1])
		if err != nil {
			t.Fatal(err)
		}
		p.tbl = tbl
		out[mi] = p
	}
	return out
}

func TestIncubateLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("incubate")
	if err != nil {
		t.Fatal(err)
	}
	prep := prepareIncubate(t, reg, set)
	const games = 1024
	// One goroutine per matchup: each game is independent (its own gorge
	// and kernel sides over the read-only prepared table), and the five
	// together keep the test well under its time budget.
	stats := make([]*incubateStats, len(prep))
	var wg sync.WaitGroup
	for mi := range prep {
		wg.Add(1)
		go func(mi int) {
			defer wg.Done()
			st := &incubateStats{carriers: map[string]int{}}
			stats[mi] = st
			p := prep[mi]
			for i := mi; i < games; i += len(prep) {
				seed := uint64(i/len(prep) + 1)
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"incubate-a", "incubate-b"}, p.decks, 1)
				if err != nil {
					t.Errorf("matchup %d seed %d: %v", mi, seed, err)
					return
				}
				k, err := SetSide(set, p.tbl, cfg)(m, seed)
				if err != nil {
					t.Errorf("matchup %d seed %d: %v", mi, seed, err)
					return
				}
				pol := incubatePolicy(seed, st)
				res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
				CloseSide(k)
				if d := res.Div; d != nil {
					lo := d.At - 12
					if lo < 0 {
						lo = 0
					}
					t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
					return
				}
			}
		}(mi)
	}
	wg.Wait()
	st := &incubateStats{carriers: map[string]int{}}
	for _, s := range stats {
		if s == nil {
			continue
		}
		st.transforms += s.transforms
		st.times += s.times
		st.doubled += s.doubled
		for n, c := range s.carriers {
			st.carriers[n] += c
		}
	}
	t.Logf("%d games, transforms %d, Times$ casts %d, doubled tokens %d, carriers %v", games, st.transforms, st.times, st.doubled, st.carriers)
	if st.transforms < 20 {
		t.Errorf("Incubator Token transformed %d times, want at least 20", st.transforms)
	}
	if st.times < 1 {
		t.Errorf("Glistening Dawn (Times$ 2) cast %d times, want at least 1", st.times)
	}
	if st.doubled < 5 {
		t.Errorf("%d tokens minted under Doubling Season, want at least 5", st.doubled)
	}
	for _, n := range incubateCarriers {
		if st.carriers[n] == 0 {
			t.Errorf("carrier %s never resolved Incubate", n)
		}
	}
}

// TestIncubateMutation: with the FaceTransform transform made a no-op
// (dirac.DebugIncubateNoTransform) the same games diverge.
func TestIncubateMutation(t *testing.T) {
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("incubate")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugIncubateNoTransform
	dirac.DebugIncubateNoTransform = true
	defer func() { dirac.DebugIncubateNoTransform = old }()
	prep := prepareIncubate(t, reg, set)
	for mi := 0; mi < len(incubateDecks); mi++ {
		p := prep[mi]
		for seed := uint64(1); seed <= 16; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"incubate-a", "incubate-b"}, p.decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, p.tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := incubatePolicy(seed, &incubateStats{carriers: map[string]int{}})
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
			CloseSide(k)
			if res.Div != nil {
				return
			}
		}
	}
	t.Fatal("no divergence with the FaceTransform transform disabled")
}
