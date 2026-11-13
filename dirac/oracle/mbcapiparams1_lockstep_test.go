package oracle

import (
	"math/rand/v2"
	"testing"

	"github.com/adams-shaun/gorge/dirac/cardset"
	"github.com/adams-shaun/gorge/dirac/kop"
	"github.com/adams-shaun/gorge/engine/cards"
	"github.com/adams-shaun/gorge/engine/state"
)

// The MBC api-params-1 locksteps (brief: one fixture-pool game per spelling).
// This first leg is the trigger ManaSpent$ / ManaNotSpent$ clause (CR 601.2h):
// "sacrifice it unless {W} was spent to cast it" (Court Hussar), "unless {U}
// was spent" (Patagia Viper), the positive "if {R} was spent, it gains haste"
// (Gruul Scrapper). gorge reads the spend in
// engine/rules/trigger_condition.go manaColourSpendHolds; the kernel binds the
// per-colour count as the trigger queues and re-reads it as the trigger
// resolves (dirac/kw_evoke.go, the Adamant gate shape). The pool table must
// carry the gate on each carrier, the games must hold in lockstep, and at
// least one game per carrier must cast with its gate colour actually spent
// (the polarity the gate suppresses on) — a kernel without the gate
// sacrifices such a permanent while gorge keeps it, so the game diverges.

var mbcSpendDeck = []cardset.Entry{
	{Name: "Court Hussar", Count: 4}, {Name: "Patagia Viper", Count: 4}, {Name: "Gruul Scrapper", Count: 4},
	{Name: "Plains", Count: 9}, {Name: "Island", Count: 8}, {Name: "Forest", Count: 6}, {Name: "Mountain", Count: 5},
}

// mbcSpendCarriers pairs each probe with its gate colour's mana index
// (state MW MU MB MR MG).
var mbcSpendCarriers = []struct {
	name   string
	colour int
}{
	{"Court Hussar", 0},   // W
	{"Patagia Viper", 1},  // U
	{"Gruul Scrapper", 3}, // R
}

const mbcSpendSeeds = 64

// mbcSpendProbe biases the seeded random policy to play a land of an
// un-tallied carrier's gate colour first and to cast the probe cards, and
// tallies which carriers it has seen cast with their gate colour spent (a
// battlefield permanent's ManaColorSpent).
type mbcSpendProbe struct {
	base Policy
	r    *rand.Rand
	seen map[string]bool
}

// mbcSpendLandColour is the basic land's colour for the land-bias.
var mbcSpendLandColour = map[string]int{"Plains": 0, "Island": 1, "Swamp": 2, "Mountain": 3, "Forest": 4}

// mbcSpendProbes is the probe set the cast-bias prefers.
var mbcSpendProbes = map[string]bool{"Court Hussar": true, "Patagia Viper": true, "Gruul Scrapper": true}

func (p *mbcSpendProbe) pick(c *PickCtx) Canon {
	// the tally is read on gorge's live state each pick
	for _, id := range c.G.E.G.Zone(state.ZBattlefield, 0) {
		ob := c.G.E.G.Obj(id)
		if ob == nil || ob.Face() == nil {
			continue
		}
		for _, cr := range mbcSpendCarriers {
			if ob.Face().Name == cr.name && ob.ManaColorSpent[cr.colour] > 0 {
				p.seen[cr.name] = true
			}
		}
	}
	a := p.base.Pick(c)
	if a.Kind() == ActPlayLand {
		// prefer a land of the first un-tallied carrier's gate colour
		for _, cr := range mbcSpendCarriers {
			if p.seen[cr.name] {
				continue
			}
			for _, x := range c.D.Options {
				if x.Kind() != ActPlayLand {
					continue
				}
				if o, ok := c.G.GorgeOption(x); ok {
					if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && mbcSpendLandColour[ob.Face().Name] == cr.colour {
						return x
					}
				}
			}
		}
		return a
	}
	if a.Kind() == ActCast {
		var probes []Canon
		for _, x := range c.D.Options {
			if x.Kind() != ActCast {
				continue
			}
			if o, ok := c.G.GorgeOption(x); ok {
				if ob := c.G.E.G.Obj(o.Obj); ob != nil && ob.Face() != nil && mbcSpendProbes[ob.Face().Name] {
					probes = append(probes, x)
				}
			}
		}
		if len(probes) > 0 && p.r.IntN(4) != 0 {
			return probes[p.r.IntN(len(probes))]
		}
	}
	return a
}

// mbcSpendRun plays seeds 1..n and reports the first divergence and the
// tallies.
func mbcSpendRun(t *testing.T, seeds uint64) (div *Divergence, seen map[string]bool) {
	t.Helper()
	cardsDir := corpusDirOrSkip(t)
	reg, err := cards.SharedCorpus(cardsDir)
	if err != nil {
		t.Fatal(err)
	}
	set, err := cardset.RawByName("mbcapiparams1")
	if err != nil {
		t.Fatal(err)
	}
	var decks [2][]*cards.Card
	for s := range decks {
		for _, e := range mbcSpendDeck {
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
	tbl, _, _, err := set.Matchup(mbcSpendDeck, mbcSpendDeck)
	if err != nil {
		t.Fatal(err)
	}
	// Precondition: each carrier's table row carries the self-ETB Adamant
	// gate of its gate colour (the ExAdamant colour|1<<3 count read kgen
	// lowers the clause to).
	gates := map[string]int{}
	for ci := range tbl.Cards {
		name := tbl.Cards[ci].Name
		for _, cr := range mbcSpendCarriers {
			if name != cr.name {
				continue
			}
			for ai := range tbl.Cards[ci].Abilities {
				cid := tbl.Cards[ci].Abilities[ai].Trig.Cond
				if cid == 0 {
					continue
				}
				cond := tbl.Conds[cid]
				if cond.Kind != kop.CondValue || cond.Op != kop.CmpEQ {
					continue
				}
				v := tbl.Exprs[cond.A.N]
				if v.Kind == kop.ExAdamant && int(v.N) == cr.colour|1<<3 {
					gates[name]++
				}
			}
		}
	}
	for _, cr := range mbcSpendCarriers {
		if gates[cr.name] == 0 {
			t.Fatalf("%s: no ManaSpend gate in the pool table", cr.name)
		}
	}
	seen = map[string]bool{}
	for seed := uint64(1); seed <= seeds && div == nil; seed++ {
		cfg := Config{GorgeAutopaySurface: seed%2 == 0}
		m, err := NewMatchup(reg, [2]string{"mbcapiparams1", "mbcapiparams1"}, decks, 1)
		if err != nil {
			t.Fatal(err)
		}
		k, err := SetSide(set, tbl, cfg)(m, seed)
		if err != nil {
			t.Fatal(err)
		}
		probe := &mbcSpendProbe{base: RandomPolicy(seed), r: rand.New(rand.NewPCG(seed, 0x6d626370)), seen: seen}
		res := Lockstep(m, seed, k, PolicyFunc(probe.pick), RunConfig{Config: cfg, MaxDecisions: 2000, MaxTurns: 20})
		CloseSide(k)
		if res.Div != nil {
			div = res.Div
			lo := div.At - 12
			if lo < 0 {
				lo = 0
			}
			t.Logf("seed %d last actions %v", seed, actionStrings(res.Actions[lo:div.At]))
			t.Errorf("seed %d: diverged at decision %d: %s %s %v %s\nref %v\nker %v", seed, div.At, div.What, div.Field, div.Diff, div.Detail, div.RefOptions, div.KerOptions)
		}
	}
	return
}

// TestMBCApiParams1Lowers is the brief's per-spelling lockstep; this leg
// covers the ManaSpent$ / ManaNotSpent$ clause. It both holds in lockstep and
// proves the gate was exercised both ways: a kernel (or gorge) without the
// gate fires the sacrifice on a gate-colour cast and the game diverges.
func TestMBCApiParams1Lowers(t *testing.T) {
	if isolate(t) {
		return
	}
	_, seen := mbcSpendRun(t, mbcSpendSeeds)
	for _, cr := range mbcSpendCarriers {
		if !seen[cr.name] {
			t.Errorf("%s: no game cast it with its gate colour spent", cr.name)
		}
	}
}
