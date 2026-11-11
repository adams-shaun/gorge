package oracle

import (
	"math/rand/v2"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The ChangeZone StaticEffect$ AddColor$ lockstep: Rise from the Grave puts
// a creature card from any graveyard onto the battlefield under its caster's
// control as a black Zombie in addition to its other colours and types
// (gorge parseStaticEffectGrant AddColor$ / AddType$, layers 4 and 5;
// dirac's override record AddColors / AddSubtypes). Doom Blade's nonblack
// filter reads the added black through its target list. The expected answer
// at every decision is gorge's.
var kriseDecks = [2][]cardset.Entry{
	{
		{Name: "Rise from the Grave", Count: 4}, {Name: "Doom Blade", Count: 3},
		{Name: "Shock", Count: 4}, {Name: "Murder", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
		{Name: "Llanowar Elves", Count: 3},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
	{
		{Name: "Rise from the Grave", Count: 3}, {Name: "Doom Blade", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Gravedigger", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 4},
		{Name: "Llanowar Elves", Count: 2},
		{Name: "Swamp", Count: 9}, {Name: "Mountain", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// kriseTally counts what the games reach, from the gorge side.
type kriseTally struct {
	rnd *rand.Rand

	risen     int // a nonblack creature card on the battlefield as a black Zombie beside its own colour
	stolen    int // ... under a player other than its owner
	bladeRead int // Doom Blade target decisions posed while a risen creature was out
	seen      map[state.ObjID]bool
}

func newKriseTally(seed uint64) *kriseTally {
	return &kriseTally{rnd: rand.New(rand.NewPCG(seed, 0x72697365)), seen: map[state.ObjID]bool{}}
}

func (w *kriseTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	risenOut := false
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Zone != state.ZBattlefield || ob.Face() == nil || ob.IsToken {
			continue
		}
		var own string
		switch ob.Face().Name {
		case "Grizzly Bears", "Llanowar Elves":
			own = "G"
		case "Hill Giant":
			own = "R"
		default:
			continue
		}
		id := state.ObjID(i + 1)
		cs := ref.E.Colors(id)
		ts, ok := ref.E.ViewTypes(id)
		if !ok || !hasWord(ts, "Zombie") || !strings.Contains(cs, "B") || !strings.Contains(cs, own) {
			continue
		}
		risenOut = true
		if !w.seen[id] {
			w.seen[id] = true
			w.risen++
			if ob.Controller != ob.Owner {
				w.stolen++
			}
		}
	}
	if d := ref.Pending(); d != nil && d.Kind == decision.KTarget && len(g.Stack) > 0 && risenOut {
		if top := g.Obj(g.Stack[len(g.Stack)-1]); top != nil && top.Face() != nil && top.Face().Name == "Doom Blade" {
			w.bladeRead++
		}
	}
	return nil
}

// policy casts whenever offered, attacks whenever it can and is random
// otherwise.
func (w *kriseTally) policy(seed uint64) Policy {
	rnd := castFirstPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if c.D.Kind == DAttackers {
			buf = buf[:0]
			for _, a := range c.D.Options {
				if a.Kind() == ActAttack {
					buf = append(buf, a)
				}
			}
			if len(buf) > 0 {
				return buf[w.rnd.IntN(len(buf))]
			}
		}
		return rnd.Pick(c)
	})
}

func (w *kriseTally) add(o *kriseTally) {
	w.risen += o.risen
	w.stolen += o.stolen
	w.bladeRead += o.bladeRead
}

// runRise plays seeds 1..seeds and returns the tally and the games that
// diverged and stalled; report logs each as a test error.
func runRise(t *testing.T, seeds uint64, report bool) (tot *kriseTally, div, stalls, games int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("risegrave")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kriseDecks {
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
	tbl, _, _, err := set.Matchup(kriseDecks[0], kriseDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tot = newKriseTally(0)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"krise-a", "krise-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		w := newKriseTally(seed)
		res := Lockstep(m, seed, k, w.policy(seed), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25, Check: w.check})
		CloseSide(k)
		games++
		tot.add(w)
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if report {
				t.Errorf("seed %d: stalled: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			div++
			if !report {
				continue
			}
			lo := max(d.At-12, 0)
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	return
}

func TestRiseFromTheGraveLockstep(t *testing.T) {
	t.Parallel()
	w, div, stalls, games := runRise(t, 128, true)
	t.Logf("%d games: %d divergences, %d stalls; risen black Zombies %d (stolen %d); Doom Blade reads with one out %d",
		games, div, stalls, w.risen, w.stolen, w.bladeRead)
	for name, n := range map[string]int{"risen": w.risen, "stolen": w.stolen, "Doom Blade reads": w.bladeRead} {
		if n < 3 {
			t.Errorf("%s: %d, want at least 3", name, n)
		}
	}
}

// TestRiseFromTheGraveMutation proves the added black is what keeps parity:
// with an AddColors record adding nothing, some seed diverges.
func TestRiseFromTheGraveMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugAnimAddColorsOff = true
	defer func() { dirac.DebugAnimAddColorsOff = false }()
	_, div, _, _ := runRise(t, 64, false)
	if div == 0 {
		t.Fatal("no seed diverged with the added colour off: the pool never reads it")
	}
	t.Logf("%d of 64 seeds diverged, as the mutation demands", div)
}
