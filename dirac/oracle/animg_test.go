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

// The Animate grants lockstep: Crawling Barrens'
// Optional$ election, Possessed Goat's additive Colors$ (black beside its
// own white), Lizard, Connors's Curse's RemoveCardTypes$ (the target keeps
// only Creature and Lizard, an Ornithopter loses Artifact), Summon: Primal
// Odin's AI-hint sVars$, and the live P/T statics Beorn's Hospitality and
// Ms. Marvel grant (SetPower$/SetToughness$ re-read at every check). Doom
// Blade reads the Goat's colour through its target list. The expected
// answer at every decision is gorge's.
var kanimgDecks = [2][]cardset.Entry{
	{
		{Name: "Crawling Barrens", Count: 4}, {Name: "Beorn's Hospitality", Count: 3},
		{Name: "Lizard, Connors's Curse", Count: 3}, {Name: "Grizzly Bears", Count: 4},
		{Name: "Llanowar Elves", Count: 4}, {Name: "Hill Giant", Count: 1},
		{Name: "Shock", Count: 2}, {Name: "Murder", Count: 1}, {Name: "Doom Blade", Count: 2},
		{Name: "Forest", Count: 8}, {Name: "Swamp", Count: 4}, {Name: "Mountain", Count: 4},
	},
	{
		{Name: "Possessed Goat", Count: 4}, {Name: "Ms. Marvel, Kamala Khan", Count: 3},
		{Name: "Summon: Primal Odin", Count: 2}, {Name: "Gravedigger", Count: 2},
		{Name: "Indigo Faerie", Count: 3}, {Name: "Cloudchaser Kestrel", Count: 2},
		{Name: "Defiant Strike", Count: 3}, {Name: "Mind Stone", Count: 1},
		{Name: "Ornithopter", Count: 2},
		{Name: "Swamp", Count: 7}, {Name: "Island", Count: 6}, {Name: "Plains", Count: 5},
	},
}

// kanimgTally counts what the games reach, from the gorge side.
type kanimgTally struct {
	rnd *rand.Rand

	askYes, askNo   int // Crawling Barrens' elections answered yes / no
	barrens         int // Barrens animated (a Creature Elemental land)
	goat            int // Possessed Goat black and white at once
	goatReads       int // Doom Blade target decisions posed while an animated Goat was out
	lizard          int // a Lizard victim exactly Creature + Lizard
	lizardArt       int // ... that was an Ornithopter (lost Artifact)
	beorn           int // Beorn's Hospitality animated
	beornPT         int // ... whose power moved with a land, counters unchanged
	marvel          int // Ms. Marvel power equal to the hand size, not the printed 1
	marvelPT        int // ... power moved while animated, counters unchanged
	odin            int // Primal Odin past chapter II
	seen            map[state.ObjID]uint8
	lastP           map[state.ObjID]int32
	lastC           map[state.ObjID]int32
	odinSeen        map[state.ObjID]bool
	lizardSeenTypes map[state.ObjID]bool
}

func newKanimgTally(seed uint64) *kanimgTally {
	return &kanimgTally{
		rnd:             rand.New(rand.NewPCG(seed, 0x616e696d67)),
		seen:            map[state.ObjID]uint8{},
		lastP:           map[state.ObjID]int32{},
		lastC:           map[state.ObjID]int32{},
		odinSeen:        map[state.ObjID]bool{},
		lizardSeenTypes: map[state.ObjID]bool{},
	}
}

const (
	seenBarrens uint8 = 1 << iota
	seenGoat
)

func hasWord(ws []string, w string) bool {
	for _, x := range ws {
		if x == w {
			return true
		}
	}
	return false
}

func (w *kanimgTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	goatOut := false
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Zone != state.ZBattlefield || ob.Face() == nil || ob.IsToken {
			continue
		}
		id := state.ObjID(i + 1)
		switch ob.Face().Name {
		case "Crawling Barrens":
			if ts, ok := ref.E.ViewTypes(id); ok && hasWord(ts, "Creature") && w.seen[id]&seenBarrens == 0 {
				w.seen[id] |= seenBarrens
				w.barrens++
			}
		case "Possessed Goat":
			if cs := ref.E.Colors(id); strings.Contains(cs, "B") && strings.Contains(cs, "W") {
				goatOut = true
				if w.seen[id]&seenGoat == 0 {
					w.seen[id] |= seenGoat
					w.goat++
				}
			}
		case "Ornithopter", "Grizzly Bears", "Llanowar Elves", "Hill Giant", "Gravedigger", "Ms. Marvel, Kamala Khan":
			if ts, ok := ref.E.ViewTypes(id); ok && hasWord(ts, "Lizard") && len(ts) == 2 && !w.lizardSeenTypes[id] {
				w.lizardSeenTypes[id] = true
				w.lizard++
				if ob.Face().Name == "Ornithopter" {
					w.lizardArt++
				}
			}
		}
		switch ob.Face().Name {
		case "Beorn's Hospitality", "Ms. Marvel, Kamala Khan":
			beorn := ob.Face().Name == "Beorn's Hospitality"
			animated := false
			if beorn {
				ts, ok := ref.E.ViewTypes(id)
				animated = ok && hasWord(ts, "Bear")
			} else {
				hand := len(g.Zone(state.ZHand, ob.Controller))
				animated = hand != 1 && int(ref.E.Power(id)) == hand
			}
			if !animated {
				delete(w.lastP, id)
				continue
			}
			p, c := ref.E.Power(id), ob.Counter("P1P1")
			if _, ok := w.lastP[id]; !ok {
				if beorn {
					w.beorn++
				} else {
					w.marvel++
				}
			} else if p != w.lastP[id] && c == w.lastC[id] {
				if beorn {
					w.beornPT++
				} else {
					w.marvelPT++
				}
			}
			w.lastP[id], w.lastC[id] = p, c
		case "Summon: Primal Odin":
			if ob.Counter("LORE") >= 3 && !w.odinSeen[id] {
				w.odinSeen[id] = true
				w.odin++
			}
		}
	}
	if d := ref.Pending(); d != nil && d.Kind == decision.KTarget && len(g.Stack) > 0 {
		if top := g.Obj(g.Stack[len(g.Stack)-1]); top != nil && top.Face() != nil && top.Face().Name == "Doom Blade" && goatOut {
			w.goatReads++
		}
	}
	return nil
}

// policy casts and activates whenever offered, attacks whenever it can,
// answers Crawling Barrens' election yes and no from its own stream, and
// is random otherwise.
func (w *kanimgTally) policy(seed uint64) Policy {
	rnd := castFirstPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil {
			if d := g.Pending(); d != nil && d.ResumeKind == "animate_optional" {
				yes, _ := chooseCode("yes")
				no, _ := chooseCode("no")
				want, ctr := ActChoose+CanonKind(no), &w.askNo
				if w.rnd.IntN(2) == 0 {
					want, ctr = ActChoose+CanonKind(yes), &w.askYes
				}
				for _, a := range c.D.Options {
					if a.Kind() == want {
						*ctr++
						return a
					}
				}
			}
		}
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

// runAnimg plays seeds 1..seeds and returns the tally, the games that
// diverged and stalled; report logs each as a test error.
func runAnimg(t *testing.T, seeds uint64, report bool) (tot *kanimgTally, div, stalls, games int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("animg")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kanimgDecks {
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
	tbl, _, _, err := set.Matchup(kanimgDecks[0], kanimgDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	tot = newKanimgTally(0)
	for seed := uint64(1); seed <= seeds; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kanimg-a", "kanimg-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		w := newKanimgTally(seed)
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

func (w *kanimgTally) add(o *kanimgTally) {
	w.askYes += o.askYes
	w.askNo += o.askNo
	w.barrens += o.barrens
	w.goat += o.goat
	w.goatReads += o.goatReads
	w.lizard += o.lizard
	w.lizardArt += o.lizardArt
	w.beorn += o.beorn
	w.beornPT += o.beornPT
	w.marvel += o.marvel
	w.marvelPT += o.marvelPT
	w.odin += o.odin
}

func TestAnimgLockstep(t *testing.T) {
	t.Parallel()
	w, div, stalls, games := runAnimg(t, 256, true)
	t.Logf("%d games: %d divergences, %d stalls; Barrens elections yes %d no %d, Barrens animated %d; Goat black+white %d, Doom Blade reads with one out %d; Lizard victims %d (Ornithopter %d); Beorn animated %d (power moved with a land %d); Ms. Marvel live power %d (moved %d); Odin past chapter II %d",
		games, div, stalls, w.askYes, w.askNo, w.barrens, w.goat, w.goatReads, w.lizard, w.lizardArt, w.beorn, w.beornPT, w.marvel, w.marvelPT, w.odin)
	for name, n := range map[string]int{
		"Barrens elections yes": w.askYes, "Barrens elections no": w.askNo, "Barrens animated": w.barrens,
		"Goat black+white": w.goat, "Doom Blade reads": w.goatReads, "Lizard victims": w.lizard,
		"Beorn animated": w.beorn, "Beorn power moves": w.beornPT, "Ms. Marvel live power": w.marvel,
		"Ms. Marvel power moves": w.marvelPT,
	} {
		if n < 3 {
			t.Errorf("%s: %d, want at least 3", name, n)
		}
	}
	if w.lizardArt < 1 {
		t.Errorf("Lizard on an Ornithopter: %d, want at least 1", w.lizardArt)
	}
	if w.odin < 1 {
		t.Errorf("Primal Odin past chapter II: %d, want at least 1", w.odin)
	}
}

// TestAnimgMutation proves the additive colour is what keeps parity: with
// an AddColors record adding nothing, some seed diverges.
func TestAnimgMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugAnimAddColorsOff = true
	defer func() { dirac.DebugAnimAddColorsOff = false }()
	_, div, _, _ := runAnimg(t, 64, false)
	if div == 0 {
		t.Fatal("no seed diverged with the additive Animate colour off: the pool never reads it")
	}
	t.Logf("%d of 64 seeds diverged, as the mutation demands", div)
}
