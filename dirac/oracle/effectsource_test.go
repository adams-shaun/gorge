package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The EffectSource lockstep: Giant Koi and Verdant
// Outrider activate an `AB$ Effect` whose CantBlockBy static reads
// ValidAttacker$ Card.EffectSource -- gorge's alias of the registered
// effect's source object (effects/filter.go). The effect's "this turn"
// lifetime and the source leaving are observably the same boundary (a
// returned card is a new object, CR 400.7), which the kernel models as the
// host-bound WhileHost emblem. Unsummon removes and replays the source so
// the boundary is exercised; the kernel must stop matching it exactly when
// gorge does.

var keffectsourceDecks = [][2][]cardset.Entry{{
	{
		{Name: "Giant Koi", Count: 4}, {Name: "Verdant Outrider", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Unsummon", Count: 4},
		{Name: "Island", Count: 7}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Giant Koi", Count: 4}, {Name: "Verdant Outrider", Count: 4},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Unsummon", Count: 4},
		{Name: "Island", Count: 7}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 4},
	},
}}

// keffectsourceTally counts the carriers' activated Effects on the gorge
// side and how often the effect's source left the battlefield in the same
// turn the effect resolved (the boundary the WhileHost emblem models).
type keffectsourceTally struct {
	pending      map[state.ObjID]string
	src          map[state.ObjID]state.ObjID
	stacked      map[string]int
	resolved     map[string]int
	activeTurn   map[state.ObjID]int32
	leftSameTurn int
}

func newKeffectsourceTally() *keffectsourceTally {
	return &keffectsourceTally{
		pending:    map[state.ObjID]string{},
		src:        map[state.ObjID]state.ObjID{},
		stacked:    map[string]int{},
		resolved:   map[string]int{},
		activeTurn: map[state.ObjID]int32{},
	}
}

func (t *keffectsourceTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	on := map[state.ObjID]bool{}
	for _, id := range g.Stack {
		o := g.Obj(id)
		if o == nil || o.Ability == nil {
			continue
		}
		on[id] = true
		if _, ok := t.pending[id]; ok {
			continue
		}
		name := ""
		if src := g.Obj(o.Source); src != nil && src.Face() != nil {
			name = src.Face().Name
		}
		key := name + "/" + o.Ability.API
		t.pending[id] = key
		t.src[id] = o.Source
		t.stacked[key]++
	}
	for id, key := range t.pending {
		if on[id] {
			continue
		}
		t.resolved[key]++
		if src := t.src[id]; src != 0 && (key == "Giant Koi/Effect" || key == "Verdant Outrider/Effect") {
			t.activeTurn[src] = g.Turn
		}
		delete(t.pending, id)
		delete(t.src, id)
	}
	for src, turn := range t.activeTurn {
		o := g.Obj(src)
		if o == nil || o.Zone != state.ZBattlefield {
			if g.Turn == turn {
				t.leftSameTurn++
			}
			delete(t.activeTurn, src)
			continue
		}
		if g.Turn != turn {
			delete(t.activeTurn, src)
		}
	}
	return nil
}

// keffectsourcePolicy prefers casting, then a battlefield activation. It
// deliberately skips hand-only activations (Giant Koi's Islandcycling) when
// the source is in hand, so the Koi reaches the battlefield to activate its
// Effect instead of being cycled away; the random stream still decides among
// same-kind options. Giant Koi/ChangeZone still counts cycling when the
// random fallback picks it.
func keffectsourcePolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActActivate {
				continue
			}
			if c.G == nil {
				buf = append(buf, a)
				continue
			}
			if o := canonObj(c, a.Src()); o != nil && o.Zone == state.ZBattlefield {
				buf = append(buf, a)
			}
		}
		if len(buf) > 0 {
			d := *c.D
			d.Options = buf
			return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		return rnd.Pick(c)
	})
}

func TestEffectsourceLockstep(t *testing.T) {
	t.Parallel()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("effectsource")
	if err != nil {
		t.Fatal(err)
	}
	tal := newKeffectsourceTally()
	games := 0
	stalls := 0
	for mi, md := range keffectsourceDecks {
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
		for seed := uint64(1); seed <= 256; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"keffectsource-a", "keffectsource-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			res := Lockstep(m, seed, k, keffectsourcePolicy(seed), RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 20, Check: tal.check})
			CloseSide(k)
			games++
			if d := res.Div; d != nil {
				lo := max(d.At-12, 0)
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
			if res.Stall != "" && strings.HasPrefix(res.Stall, KernelStallPrefix) {
				stalls++
				t.Errorf("matchup %d seed %d: stalled: %s", mi, seed, res.Stall)
			}
		}
	}
	activations := tal.resolved["Giant Koi/Effect"] + tal.resolved["Verdant Outrider/Effect"]
	t.Logf("%d games; stacked %v; resolved %v; activations %d; source left same turn %d; kernel stalls %d",
		games, tal.stacked, tal.resolved, activations, tal.leftSameTurn, stalls)
	if activations < 30 {
		t.Errorf("Effect activations = %d, want at least 30", activations)
	}
	if tal.leftSameTurn < 5 {
		t.Errorf("sources that left the battlefield the same turn = %d, want at least 5", tal.leftSameTurn)
	}
}
