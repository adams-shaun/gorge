package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The graveyard-ability activation condition lockstep:
// Balustrade Wurm and Resurrected Cultist (Activation$ Delirium, activated
// from the graveyard) and Uchbenbak, the Great Mistake (IsPresent$
// Permanent.YouOwn | PresentZone$ Graveyard | PresentCompare$ GE8, descend
// 8). gorge withholds the graveyard offer unless the ACTIVATOR's graveyard
// satisfies the condition; the kernel's cold graveActCond must do the same at
// every priority decision.

var kactgraveDecks = [2][]cardset.Entry{
	// delirium: creatures, instants, sorceries (Mulch), artifacts and an
	// aura die or are cast or milled into the graveyard
	{
		{Name: "Balustrade Wurm", Count: 4}, {Name: "Resurrected Cultist", Count: 4}, {Name: "Mulch", Count: 3},
		{Name: "Deadly Dispute", Count: 3}, {Name: "Doom Blade", Count: 3}, {Name: "Go for the Throat", Count: 2},
		{Name: "Unholy Strength", Count: 2}, {Name: "Darksteel Relic", Count: 1}, {Name: "Ornithopter", Count: 2},
		{Name: "Grizzly Bears", Count: 2}, {Name: "Llanowar Elves", Count: 2},
		{Name: "Swamp", Count: 6}, {Name: "Forest", Count: 6},
	},
	// descend 8: self-mill until eight permanent cards lie in the graveyard
	{
		{Name: "Uchbenbak, the Great Mistake", Count: 4}, {Name: "Thought Scour", Count: 4}, {Name: "Mental Note", Count: 4},
		{Name: "Tome Scour", Count: 4}, {Name: "Armored Skaab", Count: 3}, {Name: "Doom Blade", Count: 2},
		{Name: "Mind Rot", Count: 1}, {Name: "Ornithopter", Count: 1},
		{Name: "Island", Count: 9}, {Name: "Swamp", Count: 8},
	},
}

// kactgraveObs is what the games reached. Activations count the graveyard
// activations chosen (whether or not they resolve); withheld counts the
// priority windows where a delirium carrier sat in its owner's graveyard
// while that owner's graveyard held fewer than four card types.
type kactgraveObs struct {
	delirAct  int
	uchbAct   int
	withheld  int
	uchbShort int
}

var kactgraveTypes = map[string]bool{"Artifact": true, "Creature": true, "Enchantment": true, "Instant": true,
	"Land": true, "Planeswalker": true, "Sorcery": true, "Battle": true, "Kindred": true}

// kactgraveGrave returns p's graveyard card-type count and whether it holds
// a delirium carrier, and the number of permanent cards (Uchbenbak's count).
func kactgraveGrave(g *state.Game, p state.PlayerID) (types int, carrier bool, perms int, uchb bool) {
	seen := map[string]bool{}
	for _, id := range g.Zone(state.ZGraveyard, p) {
		o := g.Obj(id)
		if o == nil || o.Face() == nil {
			continue
		}
		switch o.Face().Name {
		case "Balustrade Wurm", "Resurrected Cultist":
			carrier = true
		case "Uchbenbak, the Great Mistake":
			uchb = true
		}
		perm := false
		for _, t := range o.Face().Types {
			if kactgraveTypes[string(t)] {
				seen[string(t)] = true
			}
			switch string(t) {
			case "Artifact", "Creature", "Enchantment", "Land", "Planeswalker", "Battle":
				perm = true
			}
		}
		if perm {
			perms++
		}
	}
	return len(seen), carrier, perms, uchb
}

// kactgravePolicy prefers casts and activations (graveyard ones included),
// else random, and records the observations from the gorge side.
func kactgravePolicy(seed uint64, obs *kactgraveObs) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil && c.D.Kind == DPriority {
			p := state.PlayerID(c.D.Player)
			if types, carrier, perms, uchb := kactgraveGrave(g.E.G, p); carrier && types < 4 {
				obs.withheld++
			} else if uchb && perms < 8 {
				obs.uchbShort++
			}
		}
		buf = buf[:0]
		for _, a := range c.D.Options {
			if k := a.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, a)
			}
		}
		var a Canon
		if len(buf) == 0 || seed%4 == 0 {
			a = rnd.Pick(c)
		} else {
			d := *c.D
			d.Options = buf
			a = rnd.Pick(&PickCtx{D: &d, G: c.G, Seq: c.Seq})
		}
		if a.Kind() == ActActivate && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Zone == state.ZGraveyard && ob.Face() != nil {
					switch ob.Face().Name {
					case "Balustrade Wurm", "Resurrected Cultist":
						obs.delirAct++
					case "Uchbenbak, the Great Mistake":
						obs.uchbAct++
					}
				}
			}
		}
		return a
	})
}

// runActgrave plays the constructed matchup; mutate makes the kernel ignore
// the graveyard activation condition (the mutation must diverge).
func runActgrave(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, obs *kactgraveObs) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("actgrave")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugGraveActNoCond
	dirac.DebugGraveActNoCond = mutate
	defer func() { dirac.DebugGraveActNoCond = old }()
	obs = &kactgraveObs{}
	md := kactgraveDecks
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
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kactgrave-a", "kactgrave-b"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		g, err := NewGorgeSide(m, seed, cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		res := LockstepLed(m, seed, g, k, kactgravePolicy(seed, obs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 25})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			stalls++
			if !mutate {
				t.Errorf("seed %d: %s", seed, res.Stall)
			}
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, stalls, obs
}

func TestActgraveLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, obs := runActgrave(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, delirium graveyard activations %d, windows with a delirium carrier in the graveyard and no delirium %d, Uchbenbak graveyard activations %d (windows with Uchbenbak below descend 8: %d)",
		games, divs, stalls, obs.delirAct, obs.withheld, obs.uchbAct, obs.uchbShort)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if obs.delirAct < 20 {
		t.Errorf("%d delirium carrier graveyard activations, want >= 20", obs.delirAct)
	}
	if obs.withheld < 20 {
		t.Errorf("%d windows with a delirium carrier in the graveyard and no delirium, want >= 20", obs.withheld)
	}
	if obs.uchbAct < 3 {
		t.Errorf("%d Uchbenbak graveyard activations, want >= 3", obs.uchbAct)
	}
}

// TestActgraveMutation makes the kernel ignore the graveyard ability's
// activation condition: it offers an activation gorge withholds, so the
// games must diverge.
func TestActgraveMutation(t *testing.T) {
	games, divs, _, _ := runActgrave(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation ignored the graveyard activation condition in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
