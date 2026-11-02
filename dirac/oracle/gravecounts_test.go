package oracle

import (
	"slices"
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac"
	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The graveyard card-type lockstep: Count$ValidGraveyard
// <spec>$CardTypes (Tarmogoyf's CDA over all graveyards, Broodspinner's
// tokens and Peer Past the Veil's draw over your own) and $CardTypesPermanent
// (Matzalantli, the Great Door's transform gate), and the reversed
// Count$ValidGraveyard,Battlefield zone order (Calamitous Cave-In's damage,
// Cavernous Maw's animate gate, Gargantuan Leech's cost reduction). The
// kernel's cold ExGraveAgg AggCardTypes / AggPermTypes fold decides at every
// decision the way gorge's zoneCountFold does.

var kgravecountsDecks = [2][]cardset.Entry{
	// the graveyard: creatures, an artifact creature, instants, sorceries and
	// an aura die or are cast into the graveyard; Matzalantli loots
	{
		{Name: "Tarmogoyf", Count: 4}, {Name: "Broodspinner", Count: 4}, {Name: "Peer Past the Veil", Count: 3},
		{Name: "Matzalantli, the Great Door", Count: 4}, {Name: "Shock", Count: 3}, {Name: "Ornithopter", Count: 3},
		{Name: "Mind Rot", Count: 1}, {Name: "Unholy Strength", Count: 2},
		{Name: "Swamp", Count: 5}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 5},
	},
	// the Caves: the three payoffs beside Caves that sacrifice themselves
	{
		{Name: "Calamitous Cave-In", Count: 4}, {Name: "Cavernous Maw", Count: 4}, {Name: "Gargantuan Leech", Count: 4},
		{Name: "Hidden Volcano", Count: 3}, {Name: "Hidden Nursery", Count: 3}, {Name: "Captivating Cave", Count: 2},
		{Name: "Grizzly Bears", Count: 4}, {Name: "Raging Goblin", Count: 3}, {Name: "Llanowar Elves", Count: 3},
		{Name: "Rancor", Count: 2}, {Name: "Mountain", Count: 3}, {Name: "Forest", Count: 2}, {Name: "Swamp", Count: 3},
	},
}

// kgravecountsCave reports whether p's graveyard holds a Cave card.
func kgravecountsCave(g *state.Game, p state.PlayerID) bool {
	for _, id := range g.Zone(state.ZGraveyard, p) {
		if o := g.Obj(id); o != nil && o.Face() != nil && slices.Contains(o.Face().Types, "Cave") {
			return true
		}
	}
	return false
}

// kgravecountsObs is what the games reached: Tarmogoyf's powers on the
// battlefield, the carrier plays, and the Cave payoff plays made while the
// player's graveyard held a Cave. A play counts whether or not it resolves
// (a countered or fizzled cast increments too): the metric is plays, not
// resolutions -- the weaker claim the test asserts.
type kgravecountsObs struct {
	goyf     map[int]bool
	plays    map[string]int
	cavePays int
	// caveDiscovers counts Hidden Nursery / Hidden Volcano Discover
	// activations made while the player controls a Captivating Cave: the
	// coloured pip is then payable only through the Cave's filter script
	// (seeds 6, 20, 52 diverged here on gorge).
	caveDiscovers int
}

// kgravecountsPolicy casts or activates whenever it can, else random, and
// records the observations from the gorge side.
func kgravecountsPolicy(seed uint64, obs *kgravecountsObs) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		if g := c.G; g != nil && c.D.Kind == DPriority {
			for i := range g.E.G.Objs {
				ob := &g.E.G.Objs[i]
				if ob.Zone == state.ZBattlefield && ob.Face() != nil && ob.Face().Name == "Tarmogoyf" {
					obs.goyf[int(g.E.Power(state.ObjID(i+1)))] = true
				}
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
		if k := a.Kind(); (k == ActCast || k == ActActivate) && c.G != nil {
			if o, ok := c.G.GorgeOption(a); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil {
					name := ob.Face().Name
					obs.plays[name]++
					if (name == "Hidden Nursery" || name == "Hidden Volcano") && k == ActActivate && a.Mode() == 1 {
						for i := range c.G.E.G.Objs {
							if cv := &c.G.E.G.Objs[i]; cv.Zone == state.ZBattlefield && cv.Controller == ob.Controller && cv.Face() != nil && cv.Face().Name == "Captivating Cave" {
								obs.caveDiscovers++
								break
							}
						}
					}
					switch name {
					case "Calamitous Cave-In", "Gargantuan Leech", "Cavernous Maw":
						// the casts, and Cavernous Maw's gated animate (ability 1)
						if (k == ActCast || a.Mode() == 1) && kgravecountsCave(c.G.E.G, ob.Controller) {
							obs.cavePays++
						}
					}
				}
			}
		}
		return a
	})
}

// runGravecounts plays the constructed matchup; mutate makes the
// CardTypesPermanent fold count every card type (the mutation must diverge).
func runGravecounts(t *testing.T, seeds int, mutate bool) (games, divs, stalls int, obs *kgravecountsObs) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("gravecounts")
	if err != nil {
		t.Fatal(err)
	}
	old := dirac.DebugGraveTypesAll
	dirac.DebugGraveTypesAll = mutate
	defer func() { dirac.DebugGraveTypesAll = old }()
	obs = &kgravecountsObs{goyf: map[int]bool{}, plays: map[string]int{}}
	md := kgravecountsDecks
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
		m, err := NewMatchup(reg, [2]string{"kgravecounts-a", "kgravecounts-b"}, decks, 1)
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
		res := LockstepLed(m, seed, g, k, kgravecountsPolicy(seed, obs), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
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

func TestGravecountsLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, stalls, obs := runGravecounts(t, 256, false)
	t.Logf("%d games, %d divergences, %d stalls, Tarmogoyf powers %v, Cave payoff plays with a Cave in the graveyard %d, Hidden land Discovers beside a Captivating Cave %d, plays %v",
		games, divs, stalls, obs.goyf, obs.cavePays, obs.caveDiscovers, obs.plays)
	if divs != 0 || stalls != 0 {
		t.Fatalf("%d divergences, %d stalls", divs, stalls)
	}
	if len(obs.goyf) < 3 {
		t.Errorf("Tarmogoyf reached %d distinct powers, want >= 3", len(obs.goyf))
	}
	if obs.cavePays < 20 {
		t.Errorf("%d Cave payoff plays with a Cave in the graveyard, want >= 20", obs.cavePays)
	}
	if obs.caveDiscovers == 0 {
		t.Errorf("no Hidden land Discover was activated beside a Captivating Cave: the filter-script offer was never exercised")
	}
	for _, n := range []string{"Tarmogoyf", "Broodspinner", "Peer Past the Veil", "Matzalantli, the Great Door",
		"Calamitous Cave-In", "Cavernous Maw", "Gargantuan Leech", "Captivating Cave"} {
		if obs.plays[n] == 0 {
			t.Errorf("carrier %s never played", n)
		}
	}
}

// TestGravecountsMutation makes the CardTypesPermanent fold count every
// card type: instants and sorceries open Matzalantli's transform gate
// early, so the games must diverge.
func TestGravecountsMutation(t *testing.T) {
	games, divs, _, _ := runGravecounts(t, 128, true)
	if divs == 0 {
		t.Fatalf("mutation counted every card type as a permanent type in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
