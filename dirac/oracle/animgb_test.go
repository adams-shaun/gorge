package oracle

import (
	"strconv"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/events"
	"github.com/adams-shaun/gorge/engine/state"
)

// The 22 non-Standard cards lifted as a side effect, none
// of them in a lockstep pool until now: the four Hidden and six Opal
// enchantments, Veil of Birds, Veiled Apparition and Veiled Sentry, Lurking
// Skirge (each animates itself permanently when an opponent's spell or a
// death trips it; Opal Acrolith turns back, Apparition carries an upkeep
// sacrifice-unless-pay), Ebony Fly (a die roll, then the Optional$ election),
// Hunting Wilds (kicked: the fetched Forests become 3/3 hasty creatures),
// Mercurial Transformation (Frog or Octopus), Phantasmal Form (up to two
// targets), Quirion Druid (a land becomes a 2/2), Raging Spirit (colourless),
// Scrapbasket (all colours) and Bola Slinger (backup grants its attack
// trigger). Opal Avenger and Opal Titan are refused in kgen. The expected
// answer at every decision is gorge's.
//
// No pool card is left out of the decks: Opal Acrolith's Cost$ 0 turn-back is a
// types setter that must take the newest position when re-applied, the
// turned-back counter reads gorge's view of an Acrolith that was a creature
// and is not any more, and a kicked Hunting Wilds untaps every fetched Forest.
// Raging Spirit's Colors$ Colorless is a no-op in gorge (combatfx.go fails
// closed: "colours unchanged"), so its counter is the ability being put on
// the stack, not a colour read.
var kanimgbDecks = [][2][]cardset.Entry{
	{ // M1 (G/B Hidden cards, Skirge and Druid vs U/R burn, fliers, Form and Mercurial)
		{
			{Name: "Hidden Ancients", Count: 3}, {Name: "Hidden Gibbons", Count: 3}, {Name: "Hidden Guerrillas", Count: 3},
			{Name: "Hidden Spider", Count: 3}, {Name: "Lurking Skirge", Count: 3}, {Name: "Quirion Druid", Count: 3},
			{Name: "Grizzly Bears", Count: 3}, {Name: "Hill Giant", Count: 1},
			{Name: "Forest", Count: 12}, {Name: "Swamp", Count: 6},
		},
		{
			{Name: "Shock", Count: 3}, {Name: "Lightning Bolt", Count: 3}, {Name: "Ornithopter", Count: 3},
			{Name: "Spectral Sailor", Count: 3}, {Name: "Phantasmal Form", Count: 3}, {Name: "Mercurial Transformation", Count: 3},
			{Name: "Ebony Fly", Count: 3}, {Name: "Raging Spirit", Count: 3}, {Name: "Veil of Birds", Count: 2},
			{Name: "Island", Count: 7}, {Name: "Mountain", Count: 7},
		},
	},
	{ // M2 (W/R Opal cards and Bola Slinger vs U/G Veil cards and creatures)
		{
			{Name: "Opal Archangel", Count: 3}, {Name: "Opal Caryatid", Count: 4},
			{Name: "Opal Champion", Count: 4}, {Name: "Opal Gargoyle", Count: 4}, {Name: "Opal Guardian", Count: 3},
			{Name: "Opal Acrolith", Count: 3}, {Name: "Bola Slinger", Count: 4}, {Name: "Shock", Count: 2},
			{Name: "Plains", Count: 11}, {Name: "Mountain", Count: 2},
		},
		{
			{Name: "Veil of Birds", Count: 3}, {Name: "Veiled Apparition", Count: 4}, {Name: "Veiled Sentry", Count: 4},
			{Name: "Grizzly Bears", Count: 4}, {Name: "Llanowar Elves", Count: 3}, {Name: "Spectral Sailor", Count: 3},
			{Name: "Quirion Druid", Count: 2},
			{Name: "Island", Count: 9}, {Name: "Forest", Count: 8},
		},
	},
	{ // M3 (G ramp and vanilla bodies vs R/W Scrapbasket, Raging Spirit, Bola Slinger, burn)
		{
			{Name: "Llanowar Elves", Count: 4}, {Name: "Mind Stone", Count: 3}, {Name: "Hill Giant", Count: 1},
			{Name: "Hunting Wilds", Count: 4},
			{Name: "Grizzly Bears", Count: 5}, {Name: "Forest", Count: 17}, {Name: "Mountain", Count: 6},
		},
		{
			{Name: "Scrapbasket", Count: 3}, {Name: "Raging Spirit", Count: 3}, {Name: "Bola Slinger", Count: 3},
			{Name: "Lightning Bolt", Count: 3}, {Name: "Shock", Count: 3}, {Name: "Phantasmal Form", Count: 2},
			{Name: "Mercurial Transformation", Count: 2}, {Name: "Savannah Lions", Count: 3}, {Name: "Hill Giant", Count: 2},
			{Name: "Ornithopter", Count: 1}, {Name: "Mountain", Count: 9}, {Name: "Plains", Count: 6},
		},
	},
}

// kanimgbAnimated are the cards counted once per object when they have
// become creatures.
var kanimgbAnimated = []string{
	"Hidden Ancients", "Hidden Gibbons", "Hidden Guerrillas", "Hidden Spider",
	"Opal Acrolith", "Opal Archangel", "Opal Caryatid", "Opal Champion", "Opal Gargoyle", "Opal Guardian",
	"Veil of Birds", "Veiled Apparition", "Veiled Sentry", "Lurking Skirge", "Ebony Fly",
}

// kanimgbTally counts what the games reach, from the gorge side. It embeds
// the animg tally for its election policy (Ebony Fly's Optional$ is
// answered yes and no from the same stream).
type kanimgbTally struct {
	*kanimgTally

	animated   map[string]int // kanimgbAnimated name -> objects animated
	apparSac   int            // Veiled Apparition sacrificed after animating
	apparPay   int            // the unless-pay decision of its upkeep posed
	druid      int            // a land Quirion Druid made a 2/2 creature
	huntForest int            // a Forest a kicked Hunting Wilds made a 3/3 creature
	frog       int            // Mercurial Transformation's Frog
	octopus    int            // ... and Octopus
	form0      int            // Phantasmal Form cast with no target
	form1      int            // ... one target
	form2      int            // ... two targets
	spirit     int            // Raging Spirit's colourless ability put on the stack
	scrap      int            // Scrapbasket all five colours
	bolaGrant  int            // attack triggers Bola Slinger's backup granted to another creature
	acroBack   int            // Opal Acrolith that was a creature and is not any more
	seenObj    map[state.ObjID]uint8
	formN      map[state.ObjID]int // latest target count of each Phantasmal Form seen on the stack
	apparAnim  map[state.ObjID]bool
	evAt       int
	apparAsked bool
}

const (
	kbSeenCreature uint8 = 1 << iota
	kbSeenLand
	kbWasAcro // an Opal Acrolith seen as a creature
	kbSeenAcroBack
)

func newKanimgbTally(seed uint64) *kanimgbTally {
	return &kanimgbTally{
		kanimgTally: newKanimgTally(seed),
		animated:    map[string]int{},
		seenObj:     map[state.ObjID]uint8{},
		formN:       map[state.ObjID]int{},
		apparAnim:   map[state.ObjID]bool{},
	}
}

func (w *kanimgbTally) check(ref *GorgeSide, _ Side) *Divergence {
	g := ref.E.G
	animatedNow := false
	for i := range g.Objs {
		ob := &g.Objs[i]
		if ob.Face() == nil || ob.IsToken {
			continue
		}
		id := state.ObjID(i + 1)
		name := ob.Face().Name
		if ob.Zone != state.ZBattlefield {
			if name == "Veiled Apparition" && w.apparAnim[id] && ob.Zone == state.ZGraveyard {
				w.apparAnim[id] = false
				w.apparSac++
			}
			continue
		}
		ts, ok := ref.E.ViewTypes(id)
		creature := ok && hasWord(ts, "Creature")
		switch name {
		case "Veiled Apparition":
			if creature {
				w.apparAnim[id] = true
				animatedNow = true
			}
		case "Forest", "Island", "Mountain", "Plains", "Swamp":
			if creature && w.seenObj[id]&kbSeenLand == 0 {
				w.seenObj[id] |= kbSeenLand
				switch ref.E.Power(id) {
				case 2:
					w.druid++
				case 3:
					if name == "Forest" {
						w.huntForest++
					}
				}
			}
		case "Scrapbasket":
			if cs := ref.E.Colors(id); len(cs) >= 5 && w.seenObj[id]&kbSeenCreature == 0 && strings.Contains(cs, "W") &&
				strings.Contains(cs, "U") && strings.Contains(cs, "B") && strings.Contains(cs, "R") && strings.Contains(cs, "G") {
				w.seenObj[id] |= kbSeenCreature
				w.scrap++
			}
		}
		if creature && w.seenObj[id]&kbSeenCreature == 0 {
			for _, n := range kanimgbAnimated {
				if n == name {
					w.seenObj[id] |= kbSeenCreature
					w.animated[name]++
				}
			}
		}
		if name == "Opal Acrolith" {
			if creature {
				w.seenObj[id] |= kbWasAcro
			} else if w.seenObj[id]&kbWasAcro != 0 && w.seenObj[id]&kbSeenAcroBack == 0 {
				w.seenObj[id] |= kbSeenAcroBack
				w.acroBack++
			}
		}
		if creature && w.seenObj[id]&kbSeenLand == 0 {
			switch {
			case hasWord(ts, "Frog"):
				w.seenObj[id] |= kbSeenLand
				w.frog++
			case hasWord(ts, "Octopus"):
				w.seenObj[id] |= kbSeenLand
				w.octopus++
			}
		}
	}
	for _, id := range g.Stack {
		if o := g.Obj(id); o != nil && o.Face() != nil && o.Face().Name == "Phantasmal Form" {
			w.formN[id] = len(o.Targets)
		}
	}
	if d := ref.Pending(); d != nil && d.ResumeKind == "unless_pay" && animatedNow && !w.apparAsked {
		w.apparAsked = true
		w.apparPay++
	} else if d == nil {
		w.apparAsked = false
	}
	evs := ref.E.L.Events
	if len(evs) < w.evAt {
		w.evAt = 0
	}
	for j := w.evAt; j < len(evs); j++ {
		switch e := &evs[j]; e.Kind {
		case events.AbilityPush:
			if chName(g, e.Obj) == "Raging Spirit" {
				w.spirit++
			}
		case events.GrantTriggerPush:
			// an attack trigger Bola Slinger's backup granted to another creature
			if chName(g, e.Obj) != "Bola Slinger" {
				w.bolaGrant++
			}
		}
	}
	w.evAt = len(evs)
	return nil
}

// finish settles the per-spell counts (a spell's targets are final once
// the game is over).
func (w *kanimgbTally) finish() {
	for _, n := range w.formN {
		switch n {
		case 0:
			w.form0++
		case 1:
			w.form1++
		default:
			w.form2++
		}
	}
}

func (w *kanimgbTally) add(o *kanimgbTally) {
	w.kanimgTally.add(o.kanimgTally)
	for k, v := range o.animated {
		w.animated[k] += v
	}
	w.apparSac += o.apparSac
	w.apparPay += o.apparPay
	w.druid += o.druid
	w.huntForest += o.huntForest
	w.frog += o.frog
	w.octopus += o.octopus
	w.form0 += o.form0
	w.form1 += o.form1
	w.form2 += o.form2
	w.spirit += o.spirit
	w.scrap += o.scrap
	w.bolaGrant += o.bolaGrant
	w.acroBack += o.acroBack
}

// policy is the animg policy with a throttle on activations: Opal Acrolith's
// Cost$ 0 ability is always offered, so a cast-first policy would stack it
// without end (the kernel's stack is fixed-size) and pile up layer records
// (gorge slows quadratically). An activation is taken only with an empty
// stack, one time in three, and at most kanimgbActivations times a game.
func (w *kanimgbTally) policy(seed uint64) Policy {
	inner := w.kanimgTally.policy(seed)
	acts := 0
	return PolicyFunc(func(c *PickCtx) Canon {
		a := inner.Pick(c)
		if a.Kind() != ActActivate {
			return a
		}
		if c.G == nil || len(c.G.E.G.Stack) > 0 || acts >= kanimgbActivations || w.rnd.IntN(3) != 0 {
			return passOption(c, a)
		}
		acts++
		return a
	})
}

// kanimgbActivations caps the activated abilities a game's policy takes.
const kanimgbActivations = 30

// passOption is the pass among c's options (a when there is none).
func passOption(c *PickCtx, a Canon) Canon {
	for _, o := range c.D.Options {
		if o.Kind() == ActPass {
			return o
		}
	}
	return a
}

// runAnimgb plays seeds 1..seeds of every matchup in both seatings and
// returns the tally, the games that diverged and stalled; report logs each
// as a test error.
func runAnimgb(t *testing.T, seeds uint64, report bool) (tot *kanimgbTally, div, stalls, games int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("animgb")
	if err != nil {
		t.Fatal(err)
	}
	tot = newKanimgbTally(0)
	for mi, md0 := range kanimgbDecks {
		for swap := 0; swap < 2; swap++ {
			md := md0
			if swap == 1 {
				md = [2][]cardset.Entry{md0[1], md0[0]}
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
					t.Fatalf("matchup %d deck %d: %d cards", mi, s, len(decks[s]))
				}
			}
			tbl, _, _, err := set.Matchup(md[0], md[1])
			if err != nil {
				t.Fatal(err)
			}
			for seed := uint64(1); seed <= seeds; seed++ {
				cfg := Config{GorgeAutopaySurface: seed%2 == 0}
				m, err := NewMatchup(reg, [2]string{"kanimgb-a", "kanimgb-b"}, decks, 1)
				if err != nil {
					t.Fatal(err)
				}
				k, err := SetSide(set, tbl, cfg)(m, seed)
				if err != nil {
					t.Fatal(err)
				}
				w := newKanimgbTally(seed)
				res := Lockstep(m, seed, k, w.policy(seed), RunConfig{Config: cfg, MaxDecisions: 2500, MaxTurns: 25, Check: w.check})
				CloseSide(k)
				games++
				w.finish()
				tot.add(w)
				if strings.HasPrefix(res.Stall, KernelStallPrefix) {
					stalls++
					if report {
						t.Errorf("matchup %d swap %d seed %d: stalled: %s", mi, swap, seed, res.Stall)
					}
				}
				if d := res.Div; d != nil {
					div++
					if !report {
						continue
					}
					lo := max(d.At-12, 0)
					t.Logf("matchup %d swap %d seed %d last actions %v", mi, swap, seed, actionStrings(res.Actions[lo:d.At]))
					t.Errorf("matchup %d swap %d seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", mi, swap, seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
				}
			}
		}
	}
	return
}

func TestAnimgbLockstep(t *testing.T) {
	t.Parallel()
	w, div, stalls, games := runAnimgb(t, 43, true)
	var an strings.Builder
	for _, n := range kanimgbAnimated {
		an.WriteString(" " + n + " " + strconv.Itoa(w.animated[n]) + ";")
	}
	t.Logf("%d games: %d divergences, %d stalls; animated:%s Apparition sacrificed %d, upkeep unless-pay posed %d; Fly elections yes %d no %d; Quirion Druid lands %d; Hunting Wilds Forests %d; Mercurial Frog %d Octopus %d; Phantasmal Form targets 0/1/2: %d/%d/%d; Raging Spirit colourless ability %d; Scrapbasket all colours %d; Bola granted attack triggers %d; Opal Acrolith turned back %d",
		games, div, stalls, an.String(), w.apparSac, w.apparPay, w.askYes, w.askNo, w.druid, w.huntForest, w.frog, w.octopus,
		w.form0, w.form1, w.form2, w.spirit, w.scrap, w.bolaGrant, w.acroBack)
	floors := map[string]int{
		"Apparition sacrificed": w.apparSac, "Apparition unless-pay": w.apparPay,
		"Fly elections yes": w.askYes, "Fly elections no": w.askNo, "Quirion Druid lands": w.druid, "Hunting Wilds Forests": w.huntForest,
		"Mercurial Frog": w.frog, "Mercurial Octopus": w.octopus, "Phantasmal Form 1 target": w.form1,
		"Phantasmal Form 2 targets": w.form2, "Raging Spirit colourless ability": w.spirit, "Scrapbasket all colours": w.scrap,
		"Bola granted triggers": w.bolaGrant, "Opal Acrolith turned back": w.acroBack,
	}
	for _, n := range kanimgbAnimated {
		floors[n+" animated"] = w.animated[n]
	}
	for name, n := range floors {
		if n < 3 {
			t.Errorf("%s: %d, want at least 3", name, n)
		}
	}
	// the rarest: a targetless Form
	if w.form0 < 1 {
		t.Errorf("Phantasmal Form with no target: %d, want at least 1", w.form0)
	}
}

// TestAnimgbMutation proves the additive colour is what keeps parity on
// these cards (Scrapbasket's all colours, Phantasmal Form's blue, Quirion
// Druid's and Hunting Wilds' green): with an AddColors record adding
// nothing, some seed diverges.
func TestAnimgbMutation(t *testing.T) {
	if isolate(t) {
		return
	}
	dirac.DebugAnimAddColorsOff = true
	defer func() { dirac.DebugAnimAddColorsOff = false }()
	_, div, _, games := runAnimgb(t, 8, false)
	if div == 0 {
		t.Fatal("no seed diverged with the additive Animate colour off: the pool never reads it")
	}
	t.Logf("%d of %d games diverged, as the mutation demands", div, games)
}
