package oracle

import (
	"strings"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
)

// The CounterTypes$ lockstep: the five sole-blocked
// `CounterTypes$` carriers -- one PutCounter per listed kind (gorge
// effPutCounter's OneOfEach arm, engine/effects/counters_put.go). Two
// power-up activations (Captain Marvel, Quicksilver), one ETB trigger
// (Abigale, after an Animate that removes all abilities) and two Renew
// activations from the graveyard (Champion of Dusan, Qarsi Revenant), so
// each kind's counter lands as its own instruction and the keyword counters
// (CR 122.1b) grant through every layer walk.

var ktDecks = [][2][]cardset.Entry{{
	// the W/R power-up deck: the two power-up carriers, Abigale's ETB
	// trigger and vanilla creatures to animate and target
	{
		{Name: "Captain Marvel, Earth's Protector", Count: 4}, {Name: "Quicksilver, Brash Blur", Count: 4},
		{Name: "Abigale, Eloquent First-Year", Count: 4}, {Name: "Grizzly Bears", Count: 8},
		{Name: "Plains", Count: 10}, {Name: "Mountain", Count: 10},
	},
	// the G/B Renew deck: the two Renew carriers (they need creatures in
	// the graveyard first; Shock and combat provide them), vanilla creatures
	{
		{Name: "Champion of Dusan", Count: 4}, {Name: "Qarsi Revenant", Count: 4},
		{Name: "Shock", Count: 4}, {Name: "Giant Spider", Count: 4}, {Name: "Walking Corpse", Count: 4},
		{Name: "Forest", Count: 10}, {Name: "Swamp", Count: 10},
	},
}}

// ktCarriers names each counter carrier and whether its counter ability
// resolves from a cast (Abigale's ETB trigger) or an activation.
var ktCarriers = []struct {
	name string
	act  bool
}{
	{"Captain Marvel, Earth's Protector", true},
	{"Quicksilver, Brash Blur", true},
	{"Abigale, Eloquent First-Year", false},
	{"Champion of Dusan", true},
	{"Qarsi Revenant", true},
}

// ktPolicy casts or activates whenever it can (so the games reach the
// carriers and the counter abilities resolve), else random; it counts the
// plays the gorge side poses.
func ktPolicy(seed uint64, casts, acts map[string]int) Policy {
	rnd := RandomPolicy(seed)
	var buf []Canon
	return PolicyFunc(func(c *PickCtx) Canon {
		buf = buf[:0]
		for _, o := range c.D.Options {
			if k := o.Kind(); k == ActCast || k == ActActivate {
				buf = append(buf, o)
			}
		}
		var a Canon
		if len(buf) == 0 {
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
					if k == ActCast {
						casts[name]++
					} else {
						acts[name]++
					}
				}
			}
		}
		return a
	})
}

// runCountertypes plays the constructed matchup and returns the games, the
// divergences and the carrier plays. mutate rewrites the counter kind of
// the second PutCounter instruction of each carrier's counter ability to
// +1/+1 (the mutation must diverge).
func runCountertypes(t *testing.T, seeds int, mutate bool) (games, divs int, casts, acts map[string]int) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("countertypes")
	if err != nil {
		t.Fatal(err)
	}
	md := ktDecks[0]
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
	if mutate {
		tbl = ktCopyTable(tbl)
		ktMutateCounters(tbl)
	}
	casts, acts = map[string]int{}, map[string]int{}
	for seed := uint64(1); seed <= uint64(seeds); seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"kcountertypes-a", "kcountertypes-b"}, decks, 1)
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
		pol := ktPolicy(seed, casts, acts)
		res := LockstepLed(m, seed, g, k, pol, RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		games++
		if strings.HasPrefix(res.Stall, KernelStallPrefix) {
			t.Errorf("seed %d: kernel stalled: %s", seed, res.Stall)
		}
		if d := res.Div; d != nil {
			divs++
			if !mutate {
				t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, d.At, d.What, d.Field, d.Diff, d.Detail, d.RefOptions, d.KerOptions)
			}
		}
	}
	return games, divs, casts, acts
}

// ktCopyTable deep-copies the generated table's cards so the mutation can
// edit its instructions without touching the shared pool.
func ktCopyTable(tbl *kop.Table) *kop.Table {
	cp := *tbl
	cp.Cards = append([]kop.CardDef(nil), tbl.Cards...)
	for i := range cp.Cards {
		abs := make([]kop.Ability, len(cp.Cards[i].Abilities))
		copy(abs, cp.Cards[i].Abilities)
		for j := range abs {
			if abs[j].Prog != nil {
				abs[j].Prog = append([]kop.Instr(nil), abs[j].Prog...)
			}
		}
		cp.Cards[i].Abilities = abs
	}
	return &cp
}

// ktMutateCounters rewrites the counter kind of the second PutCounter
// instruction of each carrier's counter ability to +1/+1: the listed kinds
// are what the cards place, so the games must diverge.
func ktMutateCounters(tbl *kop.Table) {
	for _, c := range ktCarriers {
		idx := -1
		for i := range tbl.Cards {
			if tbl.Cards[i].Name == c.name {
				idx = i
				break
			}
		}
		if idx < 0 {
			panic("ktMutateCounters: " + c.name + " not in the table")
		}
		changed := false
		for a := range tbl.Cards[idx].Abilities {
			n := 0
			for i := range tbl.Cards[idx].Abilities[a].Prog {
				if tbl.Cards[idx].Abilities[a].Prog[i].Op != kop.OpPutCounter {
					continue
				}
				n++
				if n == 2 {
					tbl.Cards[idx].Abilities[a].Prog[i].C = int16(kop.CtrP1P1)
					changed = true
					break
				}
			}
			if changed {
				break
			}
		}
		if !changed {
			panic("ktMutateCounters: " + c.name + " has no second PutCounter")
		}
	}
}

func TestCountertypesLockstep(t *testing.T) {
	if isolate(t) {
		return
	}

	games, divs, casts, acts := runCountertypes(t, 256, false)
	t.Logf("%d games, %d divergences, carrier casts %v, activations %v", games, divs, casts, acts)
	if divs != 0 {
		t.Fatalf("%d divergences", divs)
	}
	for _, c := range ktCarriers {
		if casts[c.name]+acts[c.name] < 20 {
			t.Errorf("carrier %s resolved %d times (casts %d, activations %d), want >= 20",
				c.name, casts[c.name]+acts[c.name], casts[c.name], acts[c.name])
		}
	}
}

// TestCountertypesMutation: with each carrier's second listed kind
// rewritten to +1/+1, the lockstep must diverge. A green mutation means the
// lockstep never reaches a mutated counter placement.
func TestCountertypesMutation(t *testing.T) {
	games, divs, _, _ := runCountertypes(t, 16, true)
	if divs == 0 {
		t.Fatalf("mutation put +1/+1 for every second kind in %d games with no divergence", games)
	}
	t.Logf("mutation diverged in %d of %d games", divs, games)
}
