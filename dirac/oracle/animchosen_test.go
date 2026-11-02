package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/decision"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The chosen-colour animation lockstep: a
// mid-resolution ChooseColor followed by Animate Colors$ ChosenColor with
// OverwriteColors$ True (gorge combatfx animateGrant reads the source's
// ChosenColor AT RESOLUTION; layer 5, CR 613.1e). Puca's Eye (permanent,
// an ETB chain) and Mondo Gecko (until end of turn, then a self Pump
// Hexproof:ChosenColor + Toxic:1 and ClearChosenColor) animate themselves;
// the colour-filtered removal Blue/Red Elemental Blast, Deathmark and
// Noxious Grasp read the animated colour through their target lists, and
// Skrelv covers the sibling chosen-colour grant family. The expected
// answer at every decision is gorge's.
var kanimchosenDecks = [2][]cardset.Entry{
	{
		// three Mondo Geckos: genState.colG holds at most three
		// chosen-colour grant records a turn, and each activation in a new
		// colour is its own record (gorge keeps each grant beside the
		// others; a fourth would StallColGrant)
		{Name: "Puca's Eye", Count: 4}, {Name: "Mondo Gecko", Count: 3},
		{Name: "Blue Elemental Blast", Count: 4}, {Name: "Red Elemental Blast", Count: 4},
		{Name: "Hill Giant", Count: 4}, {Name: "Wind Drake", Count: 4}, {Name: "Raging Goblin", Count: 3},
		{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7},
	},
	{
		{Name: "Deathmark", Count: 4}, {Name: "Noxious Grasp", Count: 2},
		{Name: "Grizzly Bears", Count: 5}, {Name: "Llanowar Elves", Count: 5},
		{Name: "Suntail Hawk", Count: 4}, {Name: "Savannah Lions", Count: 4},
		{Name: "Swamp", Count: 8}, {Name: "Plains", Count: 4}, {Name: "Forest", Count: 4},
	},
}

// kanimTally counts, from the gorge side: chosen-colour animations (a
// Choose:color event whose source then carries exactly the chosen colour)
// and colour-filtered target reads (a target decision posed while one of
// the four colour-matters removal spells resolves its targets).
type kanimTally struct {
	evAt     int
	pending  map[state.ObjID]byte // Choose:color events since the last check
	animated map[state.ObjID]bool // objects whose animation was confirmed
	anims    int                  // confirmed chosen-colour animations
	reads    int                  // target decisions of a colour-matters removal
	colTgt   int                  // ... whose options name an animated object
}

func (w *kanimTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	evs := ref.E.L.Events
	if len(evs) < w.evAt {
		w.evAt = 0
	}
	for j := w.evAt; j < len(evs); j++ {
		e := &evs[j]
		if e.Kind == events.Choose && e.Counter == "color" && len(e.Text) == 1 {
			w.pending[e.Obj] = e.Text[0]
		}
	}
	w.evAt = len(evs)
	for id, letter := range w.pending {
		o := g.Obj(id)
		if o != nil {
			if f := o.Face(); f != nil && (f.Name == "Puca's Eye" || f.Name == "Mondo Gecko") &&
				ref.E.Colors(id) == string(letter) {
				// the overwrite grant landed: the object IS the chosen
				// colour (a source that merely asked, like Skrelv, keeps
				// its printed colour and never matches)
				w.anims++
				w.animated[id] = true
			}
		}
		delete(w.pending, id)
	}
	if fr := ref.fr; fr != nil && fr.d != nil && fr.d.Kind == decision.KTarget && len(g.Stack) > 0 {
		top := g.Obj(g.Stack[len(g.Stack)-1])
		if top != nil {
			if f := top.Face(); f != nil {
				switch f.Name {
				case "Deathmark", "Noxious Grasp", "Blue Elemental Blast", "Red Elemental Blast":
					w.reads++
					for _, o := range fr.d.Options {
						if o.Obj != 0 && w.animated[o.Obj] {
							w.colTgt++
							break
						}
					}
				}
			}
		}
	}
	return nil
}

// kanimchosenPolicy is RandomPolicy with a per-turn cap of three Mondo
// Gecko activations: genState.colG holds at most three chosen-colour grant
// records a turn, while gorge keeps one grant per activation in a distinct
// colour (a fourth is a legitimate gorge line the kernel's fixed colG
// cannot hold, StallColGrant). The cap keeps the pool inside the kernel's
// declared capacity, exactly as the operator sizes pools.
func kanimchosenPolicy(seed uint64) Policy {
	rnd := RandomPolicy(seed)
	turn, act := -1, 0
	isMondo := func(g *GorgeSide, a Canon) bool {
		if g.fr == nil {
			return false
		}
		for _, pe := range g.fr.prio {
			if pe.a == a && pe.opt < len(g.fr.d.Options) &&
				strings.Contains(g.fr.d.Options[pe.opt].Label, "Mondo Gecko") {
				return true
			}
		}
		return false
	}
	return PolicyFunc(func(c *PickCtx) Canon {
		g := c.G
		if g != nil {
			if t := g.Turn(); t != turn {
				turn, act = t, 0
			}
			if c.D.Kind == DPriority && act >= 3 {
				kept := append([]Canon(nil), c.D.Options...)
				n := 0
				for _, a := range kept {
					if a.Kind() == ActActivate && isMondo(g, a) {
						continue
					}
					kept[n] = a
					n++
				}
				d := *c.D
				d.Options = kept[:n]
				c = &PickCtx{D: &d, G: g, Seq: c.Seq}
			}
		}
		a := rnd.Pick(c)
		if g != nil && a.Kind() == ActActivate && isMondo(g, a) {
			act++
		}
		return a
	})
}

func TestAnimchosenLockstep(t *testing.T) {
	t.Parallel()
	if !gorgePinAtLeast(t, gorgeDefinedKWFixTime) {
		t.Skip("gorge before cr-pump-definedkw-chosencolor (5026a0910)")
	}
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("animchosen")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s, d := range kanimchosenDecks {
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
	tbl, _, _, err := set.Matchup(kanimchosenDecks[0], kanimchosenDecks[1])
	if err != nil {
		t.Fatal(err)
	}
	w := &kanimTally{pending: map[state.ObjID]byte{}, animated: map[state.ObjID]bool{}}
	games, stalls := 0, 0
	for seed := uint64(1); seed <= 256; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kanimchosen-a", "kanimchosen-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		res := Lockstep(m, seed, k, kanimchosenPolicy(seed), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20, Check: w.check})
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			t.Errorf("seed %d: stalled: %s", seed, res.Stall)
		}
		CloseSide(k)
		games++
		if d := res.Div; d != nil {
			lo := d.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:d.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
		}
	}
	t.Logf("%d games: %d chosen-colour animations, %d colour-matters target decisions (%d naming an animated object), %d stalls",
		games, w.anims, w.reads, w.colTgt, stalls)
	if stalls != 0 {
		t.Errorf("%d games stalled", stalls)
	}
	if w.anims < 40 {
		t.Errorf("%d chosen-colour animations, want at least 40", w.anims)
	}
	if w.reads < 10 {
		t.Errorf("%d colour-matters target decisions, want at least 10", w.reads)
	}
}

// TestAnimchosenMutation proves the pool animates to the chosen colour:
// with every animation forced to the first (white) record, some seed
// diverges.
func TestAnimchosenMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	if !gorgePinAtLeast(t, gorgeDefinedKWFixTime) {
		t.Skip("gorge before cr-pump-definedkw-chosencolor (5026a0910)")
	}
	dirac.DebugAnimChosenFirst = true
	defer func() { dirac.DebugAnimChosenFirst = false }()
	if !lockstepDiverges(t, "animchosen", kanimchosenDecks, 64) {
		t.Fatal("no seed diverged with the chosen-colour animation forced white: the pool never animates")
	}
}
