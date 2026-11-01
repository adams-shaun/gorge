package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The SetState lockstep: constructed matchups over the generated setstate
// pool -- Monica Rambeau's {2}{R}{W}{W} "Transform" (gorge effSetState
// Mode$ Transform on a modal DFC: FlipFace to the next face, the same
// object) and Photon, Living Light's spell-cast counters -- so the expected
// answer at every decision is gorge's. Swords to Plowshares removes Monica
// in response to her transform, so the ability resolves on a source in
// exile: CR 701.27a leaves it on the front face there. The
// frozen FDN and BLB pair pools carry no face turn on the battlefield.

var setstateDecks = [][2][]cardset.Entry{{
	{
		{Name: "Monica Rambeau", Count: 4}, {Name: "Shoulder to Shoulder", Count: 4}, {Name: "Expedition Raptor", Count: 3},
		{Name: "Hill Giant", Count: 3}, {Name: "Gray Ogre", Count: 3}, {Name: "Swords to Plowshares", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Monica Rambeau", Count: 4}, {Name: "Goblin Diplomats", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Gray Ogre", Count: 4}, {Name: "Expedition Raptor", Count: 2}, {Name: "Swords to Plowshares", Count: 4},
		{Name: "Plains", Count: 9}, {Name: "Mountain", Count: 9},
	},
}, {
	{
		{Name: "Monica Rambeau", Count: 4}, {Name: "Shoulder to Shoulder", Count: 3}, {Name: "Gray Ogre", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Goblin Diplomats", Count: 2}, {Name: "Swords to Plowshares", Count: 4},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 9},
	},
	{
		{Name: "Monica Rambeau", Count: 3}, {Name: "Expedition Raptor", Count: 4}, {Name: "Shoulder to Shoulder", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Swords to Plowshares", Count: 4}, {Name: "Plains", Count: 11},
		{Name: "Mountain", Count: 10},
	},
}}

// setstateMinTransforms is the least number of Monica Rambeau transform
// activations each matchup must pose (a carrier that never fires proves
// nothing).
const setstateMinTransforms = 20

// setstateTransformSource returns the source object id of a Monica Rambeau
// transform ability on the stack, or 0. The stack object is the ability
// (Card nil, Source the permanent).
func setstateTransformSource(g *GorgeSide) state.ObjID {
	for _, id := range g.E.G.Stack {
		ob := g.E.G.Obj(id)
		if ob == nil || ob.Zone != state.ZStack || ob.Source == 0 {
			continue
		}
		if src := g.E.G.Obj(ob.Source); src != nil && src.Face() != nil && src.Face().Name == "Monica Rambeau" {
			return src.ID
		}
	}
	return 0
}

// setstatePolicy activates whenever it can (so the games reach the
// transform), else casts, else random; it counts the Monica activations
// the gorge side poses. In response to a transform it casts Swords to
// Plowshares at Monica, so the transform resolves on an exiled source, and
// it counts the exiles that hold Photon, Living Light (an off-field flip
// would put her there; CR 701.27a says there must be none). It also counts
// the Photons seen on the battlefield, so the test proves the in-place flip
// resolves in a real game and not only that the ability was activated.
func setstatePolicy(seed uint64, played, offField, onField *int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	seenOff := map[state.ObjID]bool{}
	seenOn := map[state.ObjID]bool{}
	var wantTarget state.ObjID
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.G != nil {
			for _, ob := range c.G.E.G.Objs {
				if ob.Face() == nil || ob.Face().Name != "Photon, Living Light" {
					continue
				}
				switch {
				case ob.Zone == state.ZExile && !seenOff[ob.ID]:
					seenOff[ob.ID] = true
					*offField++
				case ob.Zone == state.ZBattlefield && !seenOn[ob.ID]:
					seenOn[ob.ID] = true
					*onField++
				}
			}
		}
		if c.D.Kind == DTarget && wantTarget != 0 && c.G != nil {
			for _, a := range c.D.Options {
				if a.Kind() != ActTarget {
					continue
				}
				if o, ok := c.G.GorgeOption(a); ok && o.Obj == wantTarget {
					wantTarget = 0
					return a
				}
			}
			wantTarget = 0
		}
		if c.G != nil {
			if src := setstateTransformSource(c.G); src != 0 {
				for _, a := range c.D.Options {
					if a.Kind() != ActCast {
						continue
					}
					o, ok := c.G.GorgeOption(a)
					if !ok {
						continue
					}
					ob := c.G.E.G.Obj(o.Obj)
					if ob != nil && ob.Face() != nil && ob.Face().Name == "Swords to Plowshares" {
						wantTarget = src
						return a
					}
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() == ActCast && c.G != nil {
				if o, ok := c.G.GorgeOption(a); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Swords to Plowshares" {
						continue
					}
				}
			}
			buf = append(buf, a)
		}
		var a Canon
		if len(buf) == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActActivate && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && ob.Face().Name == "Monica Rambeau" {
					*played++
				}
			}
		}
		return a
	})
}

func TestSetstateLockstep(t *testing.T) {
	t.Parallel()

	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("setstate")
	if err != nil {
		t.Fatal(err)
	}
	games := 0
	for mi, md := range setstateDecks {
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
		transforms, offField, onField := 0, 0, 0
		for seed := uint64(1); seed <= 48; seed++ {
			cfg := Config{GorgeAutopaySurface: seed%2 == 0}
			m, err := NewMatchup(reg, [2]string{"setstate-a", "setstate-b"}, decks, 1)
			if err != nil {
				t.Fatal(err)
			}
			k, err := SetSide(set, tbl, cfg)(m, seed)
			if err != nil {
				t.Fatal(err)
			}
			pol := setstatePolicy(seed, &transforms, &offField, &onField)
			if seed%4 == 0 {
				pol = RandomPolicy(seed)
			}
			res := Lockstep(m, seed, k, pol, RunConfig{Config: cfg, MaxDecisions: 4000, MaxTurns: 40})
			CloseSide(k)
			games++
			if strings.HasPrefix(res.Stall, KernelStallPrefix) {
				t.Errorf("matchup %d seed %d: kernel stalled: %s", mi, seed, res.Stall)
			}
			if d := res.Div; d != nil {
				lo := d.At - 12
				if lo < 0 {
					lo = 0
				}
				t.Logf("matchup %d seed %d last actions %v", mi, seed, actionStrings(res.Actions[lo:d.At]))
				t.Errorf("matchup %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
		t.Logf("matchup %d: %d transform activations, %d on-battlefield transforms, %d off-battlefield transforms", mi, transforms, onField, offField)
		if transforms < setstateMinTransforms {
			t.Errorf("matchup %d: %d transform activations, want >= %d", mi, transforms, setstateMinTransforms)
		}
		if onField == 0 {
			t.Errorf("matchup %d: no transform resolved in place on the battlefield", mi)
		}
		if offField != 0 {
			t.Errorf("matchup %d: %d transforms resolved on a source removed in response (CR 701.27a: none)", mi, offField)
		}
	}
	t.Logf("%d games", games)
}
