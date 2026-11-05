package oracle

import (
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/state"
)

// The lockstep: constructed mirror matchups over the yuefreecast pool. Yue,
// the Moon Spirit's Waterbend {5}, {T} ability lets its controller cast a
// noncreature spell from hand without paying its mana cost (gorge effPlay:
// Valid$ Card over the controller's hand, ValidSA$ Spell.nonCreature+YouOwn,
// Optional$). The deck holds creature spells the free cast must not offer,
// Llanowar Elves as waterbend taps, and cheap instants and sorceries.

var yueDeck = []cardset.Entry{
	{Name: "Yue, the Moon Spirit", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Opt", Count: 3},
	{Name: "Divination", Count: 2}, {Name: "Grizzly Bears", Count: 4}, {Name: "Hill Giant", Count: 3},
	{Name: "Llanowar Elves", Count: 4},
	{Name: "Island", Count: 9}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 5},
}

type yueTally struct {
	activated    int // Yue activations chosen at priority (an abandoned payment retries)
	onStack      int // Yue abilities seen on top of the stack
	creatureOnly int // of those, with no noncreature spell in the controller's hand
	offered      int // free-cast asks posed by Yue
	creatureHand int // free-cast asks with a creature card in the asked hand
	creatureOpt  int // free-cast options naming a creature card (must stay 0)
	free         int // free casts chosen
	declined     int // free-cast asks declined
}

func yueName(o *state.Object) string {
	if o == nil || o.Face() == nil {
		return ""
	}
	return o.Face().Name
}

// yueHandSpell reports whether p's hand holds a noncreature nonland card,
// and whether it holds a creature card.
func yueHandSpell(g *state.Game, p state.PlayerID) (nonCreature, creature bool) {
	for _, id := range g.Zone(state.ZHand, p) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		f := o.Face()
		switch {
		case f.IsLand():
		case f.IsCreature():
			creature = true
		default:
			nonCreature = true
		}
	}
	return nonCreature, creature
}

// yuePolicy activates Yue whenever it can, else prefers casts and
// activations; seeds with seed%3 == 0 decline every free cast.
func yuePolicy(seed uint64, t *yueTally) Policy {
	rnd := RandomPolicy(seed)
	decline := seed%3 == 0
	var buf []Canon
	seen := map[state.ObjID]bool{}
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		var pd *decision.Decision
		if g != nil {
			pd = g.Pending()
			yueTop(g.E.G, seen, t)
		}
		if pd != nil && pd.ResumeKind == "play" && yueName(g.E.G.Obj(pd.Source)) == "Yue, the Moon Spirit" {
			t.offered++
			if _, cr := yueHandSpell(g.E.G, state.PlayerID(pd.Player)); cr {
				t.creatureHand++
			}
			for _, o := range pd.Options {
				if x := g.E.G.Obj(o.Obj); o.Kind == "mode" && x != nil && x.Face() != nil && x.Face().IsCreature() {
					t.creatureOpt++
				}
			}
			for _, a := range c.D.Options {
				opt, ok := g.GorgeOption(a)
				if ok && (opt.Kind == "mode") != decline {
					if decline {
						t.declined++
					} else {
						t.free++
					}
					return a
				}
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if a.Kind() != ActActivate || g == nil {
				continue
			}
			if o, ok := g.GorgeOption(a); ok && yueName(g.E.G.Obj(o.Obj)) == "Yue, the Moon Spirit" {
				t.activated++
				return a
			}
		}
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate || k == ActPlayLand {
				buf = append(buf, a)
			}
		}
		if len(buf) == 0 || seed%5 == 0 {
			return rnd.Pick(c)
		}
		d := *c.D
		d.Options = buf
		return rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
	})
}

// yueTop counts each Yue ability on top of the stack once, and whether its
// controller's hand then holds only creature spells (nothing to cast).
func yueTop(g *state.Game, seen map[state.ObjID]bool, t *yueTally) {
	if len(g.Stack) == 0 {
		return
	}
	id := g.Stack[len(g.Stack)-1]
	o := g.Obj(id)
	if o == nil || o.Face() != nil || seen[id] || yueName(g.Obj(o.Source)) != "Yue, the Moon Spirit" {
		return
	}
	seen[id] = true
	t.onStack++
	if nc, _ := yueHandSpell(g, o.Controller); !nc {
		t.creatureOnly++
	}
}

func yueRun(t *testing.T, n uint64, tally *yueTally) (div int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("yuefreecast")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range yueDeck {
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
	tbl, _, _, err := set.Matchup(yueDeck, yueDeck)
	if err != nil {
		t.Fatal(err)
	}
	for seed := uint64(1); seed <= n; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"yue-a", "yue-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, yuePolicy(seed, tally), RunConfig{Config: cfg, MaxDecisions: 3000, MaxTurns: 30})
		CloseSide(k)
		if len(res.Stall) >= len(KernelStallPrefix) && res.Stall[:len(KernelStallPrefix)] == KernelStallPrefix {
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			div++
			if div == 1 {
				t.Logf("seed %d diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return div
}

func TestYueFreeCastLockstep(t *testing.T) {
	t.Parallel()
	tally := &yueTally{}
	if div := yueRun(t, 240, tally); div != 0 {
		t.Errorf("%d divergent games", div)
	}
	t.Logf("%+v", *tally)
	if tally.onStack < 20 {
		t.Errorf("Yue's ability on the stack %d times, want at least 20", tally.onStack)
	}
	if tally.creatureOnly < 3 {
		t.Errorf("Yue's ability on the stack %d times with only creature spells in hand, want at least 3", tally.creatureOnly)
	}
	if tally.free < 5 || tally.declined < 3 {
		t.Errorf("free casts %d, declines %d, want at least 5 and 3", tally.free, tally.declined)
	}
	if tally.creatureHand < 3 {
		t.Errorf("free-cast asks with a creature card in hand: %d, want at least 3", tally.creatureHand)
	}
	if tally.creatureOpt != 0 {
		t.Errorf("free-cast asks offered a creature card %d times", tally.creatureOpt)
	}
}
